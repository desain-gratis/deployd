package deployjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"path"

	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	content_chraft "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/clickhouse-raft"
	"github.com/desain-gratis/common/lib/notifier"
	"github.com/desain-gratis/common/lib/raft"
	"github.com/rs/zerolog/log"

	"github.com/desain-gratis/deployd/src/entity"
)

const (
	TableDeploymentJob       = "deployment_job"
	TableServiceInstanceHost = "service_instance_host"

	CommandUserSubmitJob raft.Command = "deployd.user.submit-job"
	CommandUserCancelJob raft.Command = "deployd.user.cancel-job"

	// Host update
	CommandHostConfigurationUpdate raft.Command = "deployd.host.configuration-update"

	// After configured, we wait before immediately continuing
	CommandRestartConfirmation raft.Command = "deployd.restart-confirmation"

	// Update de
	CommandHostRestartServiceUpdate raft.Command = "deployd.host.restart-service-update"
)

var _ raft.Application = &raftApp{}

// raftApp / coordinator
//
// An example to do raft application's composition (aka. inheritance).
// It extends the existing "ContentApp" implementation with our business logic.
// If you have multiple, you can use actual composition instead, and then make sure the Raft App lifecycle
// is executed for each instance
type raftApp struct {
	*content_chraft.ContentApp

	topic notifier.Topic

	jobUsecase  *mycontent_base.Handler[*entity.DeploymentJob]
	serviceHost *mycontent_base.Handler[*entity.ServiceInstanceHost]
}

func New(topic notifier.Topic) *raftApp {
	stateStore := content_chraft.New(
		topic,
		content_chraft.TableConfig{Name: TableDeploymentJob, RefSize: 1, IncrementalID: true, IncrementalIDGetLimit: 10},
		content_chraft.TableConfig{Name: TableServiceInstanceHost, RefSize: 1},
	)

	jobStorage, err := stateStore.GetStorage(TableDeploymentJob)
	if err != nil {
		log.Fatal().Msgf("err: %v", err)
	}

	serviceInstanceStorage, err := stateStore.GetStorage(TableServiceInstanceHost)
	if err != nil {
		log.Fatal().Msgf("err: %v", err)
	}

	// data accessor inside raft
	jobUsecase := mycontent_base.New[*entity.DeploymentJob](jobStorage, 1)
	serviceHost := mycontent_base.New[*entity.ServiceInstanceHost](serviceInstanceStorage, 1)

	return &raftApp{
		topic:       topic,
		ContentApp:  stateStore,
		jobUsecase:  jobUsecase,
		serviceHost: serviceHost,
	}
}

// make it easier for everyone..
func (m *raftApp) OnUpdate(ctx context.Context, e raft.Entry) (raft.OnAfterApply, error) {
	switch e.Command {
	case CommandUserSubmitJob:
		// start create job
		payload, err := parseAs[entity.SubmitDeploymentJobRequest](e.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
		}
		return m.userSubmitJob(ctx, payload)
	case CommandUserCancelJob:
		// explicitly cancelling job, we cancel
		payload, err := parseAs[CancelJobRequest](e.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
		}
		return m.cancelJob(ctx, payload)
	case CommandHostConfigurationUpdate:
		// feed installation (sub)state update to raft
		payload, err := parseAs[ConfigurationUpdateRequest](e.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
		}
		return m.hostConfigurationUpdate(ctx, payload)
	case CommandRestartConfirmation:
		// if restart is confirmed, we do restart
		payload, err := parseAs[RestartConfirmation](e.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
		}
		return m.restart(ctx, payload)
	case CommandHostRestartServiceUpdate:
		// feed deployment update (sub)state update to raft
		payload, err := parseAs[HostRestartServiceUpdateRequest](e.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
		}
		return m.hostRestartServiceUpdate(ctx, payload)
	}

	// fallback to the base
	return m.ContentApp.OnUpdate(ctx, e)
}

// Because we're using Golang composition / aka inheritance, we do not need to implement the rest of raft.Application method.
// Later if we have multiple ContentApp, then you need to implement it to make sure all method are executed.

func (m *raftApp) userSubmitJob(ctx context.Context, request entity.SubmitDeploymentJobRequest) (raft.OnAfterApply, error) {
	// needto validate duplication outside raft (in http integration layer)

	// check if there is an existing deployment
	instances, err := m.serviceHost.Get(ctx, request.Service.Ns, []string{request.Service.Id}, "")
	if err != nil {
		return nil, err
	}

	if len(instances) == 0 {
		if len(request.TargetHosts) == 0 {
			return nil, errors.New("for new deployment, please specify target host")
		}

		// Create New instances

		// TODO: compare used port
		// TODO: move this to their own function
		// TODO: validate maxxing
		// TODO: accept job to modify this; (but can be very later); eg. in case of server change disk address
		randomPort := uint16(rand.UintN(16))%50000 + 14000
		uniqueReplicaID := make(map[uint64]struct{})
		for _, target := range request.TargetHosts {
			nodeHostDir := path.Join(path.Clean(target.RaftConfig.NodeHostDir), fmt.Sprintf("%v_%v", request.Service.Ns, request.Service.Id))
			walDir := path.Join(path.Clean(target.RaftConfig.WALDir), fmt.Sprintf("%v_%v", request.Service.Ns, request.Service.Id))

			if _, ok := uniqueReplicaID[target.RaftConfig.ReplicaID]; ok {
				return nil, fmt.Errorf("duplicate replica ID found: %v", target.RaftConfig.ReplicaID)
			}
			uniqueReplicaID[target.RaftConfig.ReplicaID] = struct{}{}

			instances = append(instances, &entity.ServiceInstanceHost{
				Ns:      request.Service.Ns,
				Service: request.Service.Id,
				Host:    target.Host,
				RaftConfig: &entity.RaftConfig{
					RaftPort:       randomPort,
					ReplicaID:      target.RaftConfig.ReplicaID,
					RaftWALDir:     walDir,
					NodeHostDir:    nodeHostDir, // make the same first
					RTTMillisecond: 100,         // default
				},
			})
		}
	}

	hostOrdering := make([]string, len(instances))
	hostDeploymentStatus := make(map[string]entity.HostDeploymentStatusInfo, len(instances))
	hostConfigurationStatus := make(map[string]entity.HostConfigurationStatusInfo, len(instances))
	for idx, instance := range instances {
		hostOrdering[idx] = instance.Host
		hostDeploymentStatus[instance.Host] = entity.HostDeploymentStatusInfo{
			Status: entity.HostDeploymentStatusPending,
		}
		hostConfigurationStatus[instance.Host] = entity.HostConfigurationStatusInfo{
			Status: entity.HostConfigurationStatusPending,
		}
	}

	// we create / initialize the job
	job := &entity.DeploymentJob{
		Ns:          request.Ns,
		Status:      entity.DeploymentJobStatusQueued,
		Request:     request,
		PublishedAt: request.PublishedAt,
		Deployment: entity.Deployment{
			CurrentOrder: nil, // nil since it's not yet started
			HostOrder:    hostOrdering,
			Status:       hostDeploymentStatus,
		},
		Configuration: entity.Configuration{
			Status: hostConfigurationStatus,
		},
	}

	// TODO: utilize metamaxxing
	jobMeta := map[string]any{"author": "kmg"}

	result, err := m.jobUsecase.Post(ctx, job, jobMeta)
	if err != nil {
		return nil, err
	}

	resp := SubmitJobResponse{
		SubmitJobStatus: SubmitJobStatusSuccess,
		Job:             *result,
	}

	// do something with result (eg. add validation token etc to make sure only user can update, etc.)
	// or we can leave it as it is, depends on the usecase.

	encResult, err := json.Marshal(resp)
	if err != nil {
		// server's cooked
		return nil, err
	}

	return func() (raft.Result, error) {
		if resp.SubmitJobStatus == SubmitJobStatusSuccess {
			m.topic.Broadcast(context.Background(), EventDeploymentJobCreated(resp))
		}

		return raft.Result{Value: 0, Data: encResult}, nil
	}, nil
}

func (m *raftApp) cancelJob(ctx context.Context, request CancelJobRequest) (raft.OnAfterApply, error) {
	previousJobs, err := m.jobUsecase.Get(ctx, request.Ns, []string{request.Service}, request.JobId)
	if err != nil {
		return nil, err
	}

	previousJob := previousJobs[0]

	switch previousJob.Status {
	case entity.DeploymentJobStatusCancelled:
		encResult, err := json.Marshal(previousJob)
		if err != nil {
			return nil, err
		}
		return func() (raft.Result, error) { return raft.Result{Value: 0, Data: encResult}, nil }, nil
	case entity.DeploymentJobStatusSuccess:
		return nil, fmt.Errorf("job already finished")
	}

	previousJob.Status = entity.DeploymentJobStatusCancelled

	updatedJob, err := m.jobUsecase.Post(ctx, previousJob, nil) // TODO: utilize meta. Meta have full utility here
	if err != nil {
		return nil, err
	}

	encResult, err := json.Marshal(updatedJob)
	if err != nil {
		return nil, err
	}

	return func() (raft.Result, error) { return raft.Result{Data: encResult}, nil }, nil
}

// Host configuration update
func (m *raftApp) hostConfigurationUpdate(ctx context.Context, request ConfigurationUpdateRequest) (raft.OnAfterApply, error) {
	jobs, err := m.jobUsecase.Get(ctx, request.Ns, []string{request.Service}, request.JobId)
	if err != nil {
		return nil, err
	}
	if len(jobs) != 1 {
		return nil, errors.New("job nengendi???? not found")
	}

	job := jobs[0]

	if job.Configuration.Status == nil {
		return nil, errors.New("invalid job")
	}

	if _, ok := job.Configuration.Status[request.HostName]; !ok {
		return nil, fmt.Errorf("invalid host '%v'. available hosts are: %v", request.HostName, job.Configuration.Status)
	}

	job.Configuration.Status[request.HostName] = entity.HostConfigurationStatusInfo{
		Status:       request.Status,
		ErrorMessage: request.ErrorMessage,
	}
	// TODO: dontuse serviceHost, just use the jobUsecase

	// if all host is configured; we go!!!
	allHostConfigured := true
	for _, hostConfigStatus := range job.Configuration.Status {
		allHostConfigured = allHostConfigured && hostConfigStatus.Status == entity.HostConfigurationStatusSuccess
	}

	if allHostConfigured {
		// Update the job status itself
		job.Status = entity.DeploymentJobStatusConfigured
	}

	job, err = m.jobUsecase.Post(ctx, job, nil)
	if err != nil {
		return nil, err
	}

	resp := ConfigurationUpdateResponse{Job: job, ConfirmImmediately: job.Request.IsBelieve, TriggerHost: request.HostName}
	encResult, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	return func() (raft.Result, error) {
		// This server is configured..
		m.topic.Broadcast(context.Background(), EventHostConfigured(resp))

		if allHostConfigured {
			// All server is configured ! LETS GOOO
			m.topic.Broadcast(context.Background(), EventAllHostConfigured(resp))
		}

		return raft.Result{Data: encResult}, nil
	}, nil
}

func (m *raftApp) restart(ctx context.Context, request RestartConfirmation) (raft.OnAfterApply, error) {
	jobs, err := m.jobUsecase.Get(ctx, request.Ns, []string{request.Service}, request.JobId)
	if err != nil {
		return nil, err
	}
	if len(jobs) != 1 {
		return nil, errors.New("job nengendi???? not found")
	}

	job := jobs[0]

	// only restart if job status is already CONFIGURED or DEPLOYING
	if job.Status != entity.DeploymentJobStatusConfigured && job.Status != entity.DeploymentJobStatusDeploying {
		return nil, fmt.Errorf("cannot confirm deployment. current job state is not CONFIGURED / DEPLOYING, actual: %v", job.Status)
	}

	// Initialize "deploying" stage
	if job.Status == entity.DeploymentJobStatusConfigured {
		job.Status = entity.DeploymentJobStatusDeploying

		var currentOder uint
		job.Deployment.CurrentOrder = &currentOder
		job.Deployment.ConfirmedBy = request.Agent
	}

	job, err = m.jobUsecase.Post(ctx, job, nil)
	if err != nil {
		return nil, err
	}

	step := int(*job.Deployment.CurrentOrder)
	resp := HostRestartConfirmationResponse{
		Step: step,
		Job:  *job,
	}

	if step < len(job.Deployment.HostOrder) {
		resp.TargetHost = job.Deployment.HostOrder[step] // which host that the service will restart
	}

	encResult, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	// TODO: use logicccc sequential

	return func() (raft.Result, error) {
		m.topic.Broadcast(ctx, EventRestartConfirmed(resp))
		return raft.Result{Value: 0, Data: encResult}, nil
	}, nil
}

func (m *raftApp) hostRestartServiceUpdate(ctx context.Context, request HostRestartServiceUpdateRequest) (raft.OnAfterApply, error) {
	jobs, err := m.jobUsecase.Get(ctx, request.Ns, []string{request.Service}, request.JobId)
	if err != nil {
		return nil, err
	}
	if len(jobs) != 1 {
		return nil, errors.New("job nengendi???? not found")
	}

	job := jobs[0]

	if job.Status != entity.DeploymentJobStatusDeploying {
		return nil, errors.New("invalid state")
	}

	hostOnProgress := job.Deployment.HostOrder[*job.Deployment.CurrentOrder]
	if hostOnProgress != request.HostName {
		// or other meaningful error based on deployed host...
		return nil, fmt.Errorf("host %v is not yet on deployment. Please wait for %v", request.HostName, hostOnProgress)
	}

	job.Deployment.Status[request.HostName] = entity.HostDeploymentStatusInfo{
		Status:       request.Status,
		ErrorMessage: request.ErrorMessage,
	}

	// If one fail, then we fail the whole job
	if request.Status == entity.HostDeploymentStatusFailed {
		job.Status = entity.DeploymentJobStatusFailed
		job, err = m.jobUsecase.Post(ctx, job, nil)
		if err != nil {
			return nil, err
		}

		resp := HostRestartServiceUpdateResponse{
			Failed:     true,
			FailReason: request.ErrorMessage,
		}

		encResult, err := json.Marshal(resp)
		if err != nil {
			// server's cooked
			return nil, err
		}

		return func() (raft.Result, error) {
			m.topic.Broadcast(context.Background(), EventDeploymentFailed(resp))
			return raft.Result{Data: encResult, Value: 0}, nil
		}, nil
	}

	// If it's other status than success, we just update; a FYI
	// General update to the state..
	if request.Status != entity.HostDeploymentStatusSuccess {
		job, err = m.jobUsecase.Post(ctx, job, nil)
		if err != nil {
			return nil, err
		}

		resp := HostRestartServiceUpdateResponse{
			Job:         *job,
			TriggerHost: request.HostName,
		}

		encResult, err := json.Marshal(resp)
		if err != nil {
			// server's cooked
			return nil, err
		}

		return func() (raft.Result, error) {
			m.topic.Broadcast(context.Background(), resp)
			return raft.Result{Data: encResult, Value: 0}, nil
		}, nil
	}

	// NOW, the real deal; if it's success.

	*job.Deployment.CurrentOrder++

	// It means, all restart are successful.
	if int(*job.Deployment.CurrentOrder) >= len(job.Deployment.Status) {
		job.Status = entity.DeploymentJobStatusDeployed

		job, err = m.jobUsecase.Post(ctx, job, nil)
		if err != nil {
			return nil, err
		}

		resp := HostRestartServiceUpdateResponse{
			Job:         *job,
			TriggerHost: request.HostName,
		}

		encResult, err := json.Marshal(resp)
		if err != nil {
			// server's cooked
			return nil, err
		}

		return func() (raft.Result, error) {
			// notify the good news
			m.topic.Broadcast(context.Background(), EventServiceRestarted(resp))
			m.topic.Broadcast(context.Background(), EventAllServiceRestarted(resp))
			return raft.Result{Data: encResult, Value: 0}, nil
		}, nil
	}

	job, err = m.jobUsecase.Post(ctx, job, nil)
	if err != nil {
		return nil, err
	}

	resp := HostRestartServiceUpdateResponse{
		DeployImmediately: job.Request.IsBelieve,
		Job:               *job,
		TriggerHost:       request.HostName,
	}

	encResult, err := json.Marshal(job)
	if err != nil {
		// server's cooked
		return nil, err
	}

	return func() (raft.Result, error) {
		// This server is configured..
		m.topic.Broadcast(context.Background(), EventServiceRestarted(resp))

		return raft.Result{Data: encResult}, nil
	}, nil
}

func parseAs[T any](payload []byte) (T, error) {
	var t T
	err := json.Unmarshal(payload, &t)
	return t, err
}
