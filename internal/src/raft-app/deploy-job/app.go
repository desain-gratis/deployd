package deployjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	content_badger "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/badger"
	"github.com/desain-gratis/common/lib/notifier"
	"github.com/desain-gratis/common/lib/raft"
	raft_utility "github.com/desain-gratis/common/lib/raft/utility"
	"github.com/dgraph-io/badger/v4"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/rs/zerolog/log"

	"github.com/desain-gratis/deployd/src/entity"
)

type Command string

const (
	TableDeploymentJob     = "deploy_job__deployment_job"
	TableDeploymentSuccess = "deploy_job__deployment_success"

	CommandUserSubmitJob Command = "deployd.user.submit-job"
	CommandUserCancelJob Command = "deployd.user.cancel-job"

	// Host update
	CommandHostConfigurationUpdate Command = "deployd.host.configuration-update"

	// After configured, we wait before immediately continuing
	CommandRestartConfirmation Command = "deployd.restart-confirmation"

	// Update de
	CommandHostRestartServiceUpdate Command = "deployd.host.restart-service-update"
)

// var _ raft.Application = &raftApp{}
var _ raft.ApplicationV2 = (*raftApp)(nil)

// raftApp / coordinator
//
// An example to do raft application's composition (aka. inheritance).
// It extends the existing "ContentApp" implementation with our business logic.
// If you have multiple, you can use actual composition instead, and then make sure the Raft App lifecycle
// is executed for each instance
type raftApp struct {
	topic notifier.Topic

	jobCache *expirable.LRU[jobKey, *entity.DeploymentJob]

	jobUsecase           *mycontent_base.Handler[*entity.DeploymentJob]
	jobLatestUsecase     *mycontent_base.Handler[*entity.DeploymentJob]
	successfulJobUsecase *mycontent_base.Handler[*entity.DeploymentJobByService]

	mw *raft_utility.BadgerMetadataWriter
}

type jobKey struct {
	namespace string
	service   string
	id        string
}

type CommandWrapper struct {
	Name  Command `json:"name"`
	Value []byte  `json:"value"`
}

type ApplyResult func() (any, error)

// TODO: migrate this raft application to V2 maxxxiingg

// ErrRetryable crashes the state machine to preserve state invariant
var ErrRetryable = errors.New("retryable")

func New(topic notifier.Topic) *raftApp {

	// this app have its own storage, but exposes the mycontent interface via Get.. for external viewing.

	optsJob := badger.DefaultOptions("").WithInMemory(true)
	dbJob, err := badger.Open(optsJob)
	if err != nil {
		log.Fatal().Msgf("UHUY: %v", err)
	}

	jobStorage := content_badger.NewAutoIncrement(dbJob, TableDeploymentJob, 1)
	jobLatestStorage := jobStorage.GetLatest()
	serviceInstanceStorage := content_badger.New(dbJob, TableDeploymentSuccess, 0)

	// data accessor inside raft
	jobUsecase := mycontent_base.New[*entity.DeploymentJob](jobStorage)
	jobLatestUsecase := mycontent_base.New[*entity.DeploymentJob](jobLatestStorage)
	successfulJobUsecase := mycontent_base.New[*entity.DeploymentJobByService](serviceInstanceStorage)

	// cache is important because we don't rely on DB for get-and-set operation (expect stale data, trade off with high write)
	jobCache := expirable.NewLRU[jobKey, *entity.DeploymentJob](256, nil, 20*time.Minute) // at least until the DB can catch up

	return &raftApp{
		topic:                topic,
		jobUsecase:           jobUsecase,
		jobLatestUsecase:     jobLatestUsecase,
		successfulJobUsecase: successfulJobUsecase,
		jobCache:             jobCache,
		mw:                   raft_utility.NewBadgerMetadataWriter(dbJob, "deploy-job-last-applied-index"),
	}
}

func (m *raftApp) GetJobStore() *mycontent_base.Handler[*entity.DeploymentJob] {
	// todo: make view only
	return m.jobUsecase
}

func (m *raftApp) GetJobLatestUsecase() *mycontent_base.Handler[*entity.DeploymentJob] {
	// todo: make view only
	return m.jobLatestUsecase
}

func (m *raftApp) GetSuccessfulJobStore() *mycontent_base.Handler[*entity.DeploymentJobByService] {
	return m.successfulJobUsecase
}

// todo: improve pattern
// evaluate and apply
func (m *raftApp) apply(ctx context.Context, entry raft.EntryV2, result ApplyResult, err error) (any, error) {
	if errors.Is(err, ErrRetryable) {
		log.Fatal().Msgf("crashing the state machine: %v", err)
	}

	errApply := m.mw.Apply(ctx, entry)
	if errApply != nil {
		log.Fatal().Msgf("crashing the state machine (meta): %v", err)
	}

	if err != nil {
		return nil, err
	}

	if result == nil {
		result = func() (any, error) {
			return "success", nil
		}
	}

	// the response we want from user
	return result()
}

func (m *raftApp) OnUpdateV2(ctx context.Context, entry raft.EntryV2) (any, error) {
	cmd, err := parseAs[CommandWrapper](entry.Data)
	if err != nil {
		defer m.mw.Apply(ctx, entry) // bad data, ignore
		return nil, err
	}

	switch cmd.Name {
	case CommandUserSubmitJob:
		// start create job
		payload, err := parseAs[entity.SubmitDeploymentJobRequest](cmd.Value)
		if err != nil {
			defer m.mw.Apply(ctx, entry) // bad data, ignore
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(cmd.Value))
		}
		result, err := m.userSubmitJob(ctx, payload)
		return m.apply(ctx, entry, result, err)
	case CommandUserCancelJob:
		// explicitly cancelling job, we cancel
		payload, err := parseAs[CancelJobRequest](cmd.Value)
		if err != nil {
			defer m.mw.Apply(ctx, entry) // bad data, ignore
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(cmd.Value))
		}
		result, err := m.cancelJob(ctx, payload)
		return m.apply(ctx, entry, result, err)
	case CommandHostConfigurationUpdate:
		// feed installation (sub)state update to raft
		payload, err := parseAs[ConfigurationUpdateRequest](cmd.Value)
		if err != nil {
			defer m.mw.Apply(ctx, entry) // bad data, ignore
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(cmd.Value))
		}
		result, err := m.applyHostConfigurationUpdate(ctx, payload)
		return m.apply(ctx, entry, result, err)
	case CommandRestartConfirmation:
		// if restart is confirmed, we do restart
		payload, err := parseAs[RestartConfirmation](cmd.Value)
		if err != nil {
			defer m.mw.Apply(ctx, entry) // bad data, ignore
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(cmd.Value))
		}
		result, err := m.restartHostService(ctx, payload)
		return m.apply(ctx, entry, result, err)
	case CommandHostRestartServiceUpdate:
		// feed deployment update (sub)state update to raft
		payload, err := parseAs[HostRestartServiceUpdateRequest](cmd.Value)
		if err != nil {
			defer m.mw.Apply(ctx, entry) // bad data, ignore
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(cmd.Value))
		}
		result, err := m.applyHostRestartServiceUpdate(ctx, payload)
		return m.apply(ctx, entry, result, err)
	default:
		defer m.mw.Apply(ctx, entry) // bad data, ignore
		return nil, fmt.Errorf("unknown command: %s", cmd.Name)
	}
}

func (m *raftApp) InitV2(ctx context.Context) (uint64, error) {
	return m.mw.GetLastAppliedIndex(ctx)
}

// func (m *raftApp) OnUpdate(ctx context.Context, e raft.Entry) (raft.OnAfterApply, error) {
// 	// make it easier for everyone..
// 	switch Command(e.Command) {
// 	case CommandUserSubmitJob:
// 		// start create job
// 		payload, err := parseAs[entity.SubmitDeploymentJobRequest](e.Value)
// 		if err != nil {
// 			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
// 		}
// 		return m.userSubmitJob(ctx, payload)
// 	case CommandUserCancelJob:
// 		// explicitly cancelling job, we cancel
// 		payload, err := parseAs[CancelJobRequest](e.Value)
// 		if err != nil {
// 			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
// 		}
// 		return m.cancelJob(ctx, payload)
// 	case CommandHostConfigurationUpdate:
// 		// feed installation (sub)state update to raft
// 		payload, err := parseAs[ConfigurationUpdateRequest](e.Value)
// 		if err != nil {
// 			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
// 		}
// 		return m.applyHostConfigurationUpdate(ctx, payload)
// 	case CommandRestartConfirmation:
// 		// if restart is confirmed, we do restart
// 		payload, err := parseAs[RestartConfirmation](e.Value)
// 		if err != nil {
// 			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
// 		}
// 		return m.restartHostService(ctx, payload)
// 	case CommandHostRestartServiceUpdate:
// 		// feed deployment update (sub)state update to raft
// 		payload, err := parseAs[HostRestartServiceUpdateRequest](e.Value)
// 		if err != nil {
// 			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
// 		}
// 		return m.applyHostRestartServiceUpdate(ctx, payload)
// 	}

// 	// fallback to the base
// 	return func() (raft.Result, error) {
// 		return raft.Result{}, nil
// 	}, nil
// }

// Because we're using Golang composition / aka inheritance, we do not need to implement the rest of raft.Application method.
// Later if we have multiple ContentApp, then you need to implement it to make sure all method are executed.

func (m *raftApp) userSubmitJob(ctx context.Context, request entity.SubmitDeploymentJobRequest) (ApplyResult, error) {
	// TODO: make sure there is no active deployment for the namespace/service pair.
	var previousJob *entity.DeploymentJob
	previousSuccessfulDeployment, err := m.successfulJobUsecase.Get(ctx, request.Service.Ns, nil, request.Service.Id)
	if err != nil && !errors.Is(err, mycontent.ErrNotFound) {
		return nil, fmt.Errorf("failed to get previous successful job: %w", err)
	}
	if len(previousSuccessfulDeployment) >= 1 {
		previousJob = previousSuccessfulDeployment[0].DeploymentJob
	}

	deploymentTarget, err := m.getDeploymentTarget(previousJob, request)
	if err != nil {
		return nil, err
	}

	raftShards, err := m.getRaftShardsConfig(previousJob, deploymentTarget, request)
	if err != nil {
		return nil, err
	}

	raftServiceConfig, err := m.getRaftServiceConfig(previousJob, deploymentTarget, request)
	if err != nil {
		return nil, err
	}

	// ETCD
	raftEtcdConfig := make(map[string]entity.EtcdRaftReplicaConfig)
	for _, replica := range request.EtcdRaftReplicas {
		raftEtcdConfig[replica] = entity.EtcdRaftReplicaConfig{
			// assigned port within the cluster internal address
			AssignedPort: randomPort(), // ofcors todo add avaoid conflict logic
			Join:         false,        // todo: calculated with last successful job
		}
	}

	job := entity.DeploymentJob{
		Ns:          request.Ns,
		PublishedAt: request.PublishedAt,
	}
	job.Target = deploymentTarget
	job.RaftConfig = &entity.RaftConfig{
		Shards:           raftShards,
		Service:          raftServiceConfig,
		EtcdRaftReplicas: raftEtcdConfig,
	}
	job.Request = &request

	// for convenience
	// job.Request.TargetHosts = nil

	initDeploymentJobState(&job)

	// TODO: utilize metamaxxing
	jobMeta := map[string]any{"author": "kmg"}

	result, err := m.jobUsecase.Post(ctx, &job, jobMeta)
	if err != nil {
		return nil, err
	}

	kc := jobKey{namespace: job.Ns, service: job.Request.Service.Id, id: job.Id}
	_ = m.jobCache.Add(kc, &job)

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

	return func() (any, error) {
		m.topic.Broadcast(context.Background(), EventDeploymentJobCreated(resp))
		return encResult, nil
	}, nil
}

func (m *raftApp) cancelJob(ctx context.Context, request CancelJobRequest) (ApplyResult, error) {
	previousJob, err := m.getJobByID(ctx, request.Ns, request.Service, request.JobId)
	if err != nil {
		return nil, err
	}

	switch previousJob.Status {
	case entity.DeploymentJobStatusCancelled:
		encResult, err := json.Marshal(previousJob)
		if err != nil {
			return nil, err
		}
		return func() (any, error) { return raft.Result{Value: 0, Data: encResult}, nil }, nil
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

	return func() (any, error) {
		return encResult, nil
	}, nil
}

// Host configuration update
func (m *raftApp) applyHostConfigurationUpdate(ctx context.Context, request ConfigurationUpdateRequest) (ApplyResult, error) {
	job, err := m.getJobByID(ctx, request.Ns, request.Service, request.JobId)
	if err != nil {
		return nil, err
	}

	if job.ConfigureHostJob.Status == nil {
		return nil, errors.New("invalid job")
	}

	if _, ok := job.ConfigureHostJob.Status[request.HostName]; !ok {
		return nil, fmt.Errorf("invalid host '%v'. available hosts are: %v", request.HostName, job.ConfigureHostJob.Status)
	}

	job.ConfigureHostJob.Status[request.HostName] = entity.HostConfigurationState{
		Status:       request.Status,
		ErrorMessage: request.ErrorMessage,
	}

	// TODO: more elegant state handling and host timeout. (use leader)

	// if all host is configured; we go!!!
	allHostConfigured := true
	markFailed := false
	var failReason *string
	for _, hostConfigStatus := range job.ConfigureHostJob.Status {
		allHostConfigured = allHostConfigured && hostConfigStatus.Status == entity.HostConfigurationStatusSuccess
		markFailed = markFailed || job.ConfigureHostJob.Status[request.HostName].Status == entity.HostConfigurationStatusFailed
	}

	if allHostConfigured {
		// Update the job status itself
		job.Status = entity.DeploymentJobStatusConfigured

		if !job.Request.IsBelieve {
			// if front end saw this / user interface web found this message, it should open the dialog for confirming deployment
			job.PendingUserAction.ConfirmDeployment = &entity.ConfirmDeployment{
				Message:        "Target hosts are configured. Proceed with deployment?",
				CTAButtonLabel: "CONTINUE",
			}
		}
	} else if markFailed {
		job.Status = entity.DeploymentJobStatusFailed
		msg := "a host failed configuration"
		failReason = &msg
	} else {
		job.Status = entity.DeploymentJobStatusConfiguring
	}

	job, err = m.jobUsecase.Post(ctx, job, nil)
	if err != nil {
		return nil, err
	}

	resp := ConfigurationUpdateResponse{
		Job:         *job,
		TriggerHost: request.HostName,
		Failed:      markFailed,
		FailReason:  failReason,
	}
	encResult, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	return func() (any, error) {
		// This server is configured..
		if job.ConfigureHostJob.Status[request.HostName].Status == entity.HostConfigurationStatusSuccess {
			// it's configured
			m.topic.Broadcast(context.Background(), EventHostConfigured(resp))

			if allHostConfigured {
				// All server is configured ! LETS GOOO
				m.topic.Broadcast(context.Background(), EventAllHostConfigured(resp))
			}
		} else {
			// ordinary update
			m.topic.Broadcast(context.Background(), EventHostConfigurationUpdate(resp))
		}

		if markFailed {
			m.topic.Broadcast(context.Background(), EventDeploymentJobFailed(resp))
		}

		return encResult, nil
	}, nil
}

func (m *raftApp) restartHostService(ctx context.Context, request RestartConfirmation) (ApplyResult, error) {
	job, err := m.getJobByID(ctx, request.Ns, request.Service, request.JobId)
	if err != nil {
		return nil, err
	}

	// only restart if job status is already CONFIGURED or DEPLOYING
	if job.Status != entity.DeploymentJobStatusConfigured && job.Status != entity.DeploymentJobStatusDeploying {
		return nil, fmt.Errorf("cannot confirm deployment. current job state is not CONFIGURED / DEPLOYING, actual: %v", job.Status)
	}

	// Initialize "deploying" stage
	if job.Status == entity.DeploymentJobStatusConfigured {
		job.Status = entity.DeploymentJobStatusDeploying

		var currentOder uint
		job.RestartServiceJob.CurrentOrder = &currentOder
		job.RestartServiceJob.ConfirmedBy = request.Agent
	}

	job, err = m.jobUsecase.Post(ctx, job, nil)
	if err != nil {
		return nil, err
	}

	step := int(*job.RestartServiceJob.CurrentOrder)
	resp := HostRestartConfirmationResponse{
		Step: step,
		Job:  *job,
	}

	if step < len(job.RestartServiceJob.HostOrder) {
		resp.TargetHost = job.RestartServiceJob.HostOrder[step] // which host that the service will restart
	}

	encResult, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	// TODO: use logicccc sequential

	return func() (any, error) {
		m.topic.Broadcast(ctx, EventRestartConfirmed(resp))
		return encResult, nil
	}, nil
}

func (m *raftApp) applyHostRestartServiceUpdate(ctx context.Context, request HostRestartServiceUpdateRequest) (ApplyResult, error) {
	job, err := m.getJobByID(ctx, request.Ns, request.Service, request.JobId)
	if err != nil {
		return nil, err
	}

	if job.Status != entity.DeploymentJobStatusDeploying {
		return nil, errors.New("invalid state")
	}

	hostOnProgress := job.RestartServiceJob.HostOrder[*job.RestartServiceJob.CurrentOrder]
	if hostOnProgress != request.HostName {
		// or other meaningful error based on deployed host...
		return nil, fmt.Errorf("host %v is not yet on deployment. Please wait for %v", request.HostName, hostOnProgress)
	}

	job.RestartServiceJob.Status[request.HostName] = entity.HostRestartServiceState{
		Status:       request.Status,
		ErrorMessage: request.ErrorMessage,
	}

	// If one fail, then we fail the whole job
	if request.Status == entity.HostRestartServiceStatusFailed {
		job.Status = entity.DeploymentJobStatusFailed
		job, err = m.jobUsecase.Post(ctx, job, nil)
		if err != nil {
			return nil, err
		}

		resp := HostRestartServiceUpdateResponse{
			CommonResponse: CommonResponse{
				Job:        *job,
				Failed:     true,
				FailReason: request.ErrorMessage,
			},
		}

		encResult, err := json.Marshal(resp)
		if err != nil {
			// server's cooked
			return nil, err
		}

		return func() (any, error) {
			m.topic.Broadcast(context.Background(), EventDeploymentJobFailed(resp.CommonResponse))
			return encResult, nil
		}, nil
	}

	// If it's other status than success, we just update; a FYI
	// General update to the state..
	if request.Status != entity.HostRestartServiceStatusSuccess {
		job, err = m.jobUsecase.Post(ctx, job, nil)
		if err != nil {
			return nil, err
		}

		resp := HostRestartServiceUpdateResponse{
			CommonResponse: CommonResponse{
				Job:         *job,
				TriggerHost: request.HostName,
			},
		}

		encResult, err := json.Marshal(resp)
		if err != nil {
			// server's cooked
			return nil, err
		}

		return func() (any, error) {
			m.topic.Broadcast(context.Background(), EventServiceRestartUpdate(resp))
			return encResult, nil
		}, nil
	}

	// NOW, the real deal; if it's success.

	*job.RestartServiceJob.CurrentOrder++

	// It means, all restart are successful.
	if int(*job.RestartServiceJob.CurrentOrder) >= len(job.RestartServiceJob.Status) {
		job.Status = entity.DeploymentJobStatusDeployed
		job.FinishedAt = &request.UpdatedAt

		job, err = m.jobUsecase.Post(ctx, job, nil)
		if err != nil {
			return nil, err
		}

		jobCopy := *job
		jobCopy.Id = jobCopy.Request.Service.Id // index not by version, but by ID

		_, err = m.successfulJobUsecase.Post(ctx, &entity.DeploymentJobByService{DeploymentJob: &jobCopy}, nil)
		if err != nil {
			return nil, err
		}

		resp := HostRestartServiceUpdateResponse{
			CommonResponse: CommonResponse{
				Job:         *job,
				TriggerHost: request.HostName,
			},
		}

		encResult, err := json.Marshal(resp)
		if err != nil {
			// server's cooked
			return nil, err
		}

		return func() (any, error) {
			// notify the good news
			m.topic.Broadcast(context.Background(), EventServiceRestarted(resp))
			m.topic.Broadcast(context.Background(), EventAllServiceRestarted(resp))
			m.topic.Broadcast(context.Background(), EventDeploymentJobSuccess(resp.CommonResponse))
			return encResult, nil
		}, nil
	}

	// if not, we just save the successful restart update, and continue

	job, err = m.jobUsecase.Post(ctx, job, nil)
	if err != nil {
		return nil, err
	}

	resp := HostRestartServiceUpdateResponse{
		CommonResponse: CommonResponse{
			Job:         *job,
			TriggerHost: request.HostName,
		},
	}

	encResult, err := json.Marshal(job)
	if err != nil {
		// server's cooked
		return nil, err
	}

	return func() (any, error) {
		// This server is configured;
		m.topic.Broadcast(context.Background(), EventServiceRestarted(resp))

		return encResult, nil
	}, nil
}

func getLeaderByTarget(target []entity.Host) (entity.Host, error) {
	ntarget := len(target)
	if ntarget <= 1 {
		return entity.Host{}, fmt.Errorf("invalid deployment target length %v", len(target))
	}

	return target[len(target)-1], nil
}

func (m *raftApp) getRaftServiceConfig(previousJob *entity.DeploymentJob, target []entity.Host, request entity.SubmitDeploymentJobRequest) (map[string]entity.RaftServiceConfig, error) {
	if previousJob != nil {
		// always use previous config (because it's already "baked")
		// change in configuration is possible, but only with new deployment
		return previousJob.RaftConfig.Service, nil
	}

	if request.RaftPort <= 10000 { // || request.RaftPort >= math.MaxUint16
		return nil, fmt.Errorf("reqeusted raft port: %v is outside of  valid range (10000 - 65535)", request.RaftPort)
	}

	uniqValidation := make(map[uint64]string)
	var err error
	result := make(map[string]entity.RaftServiceConfig)
	for _, t := range target {
		// TODO: validatemaxxing
		if conflict, ok := uniqValidation[t.RaftConfig.ReplicaID]; ok {
			err = errors.Join(err, fmt.Errorf("duplicate replica id=%v for host=%v and host=%v", t.RaftConfig.ReplicaID, t.Host, conflict))
		}

		result[t.Host] = entity.RaftServiceConfig{
			RaftAddress:  fmt.Sprintf("%v:%d", t.Host, request.RaftPort),
			ReplicaID:    t.RaftConfig.ReplicaID,   // get from host for convenience
			DeploymentID: request.RaftDeploymentID, // TODO: implement

			// The others can be inffered from host's t.RaftConfig
			// from service perspective, only raft port is needed to be agreed
		}
		uniqValidation[t.RaftConfig.ReplicaID] = t.Host
	}
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (m *raftApp) getRaftShardsConfig(previousJob *entity.DeploymentJob, target []entity.Host, request entity.SubmitDeploymentJobRequest) (map[uint64]entity.RaftShardConfig, error) {
	bootstrapHost, err := getLeaderByTarget(target)
	if err != nil {
		return nil, err
	}

	replica := make(map[uint64]entity.RaftShardConfig)

	// TODO: detect changes with previous job

	for shardID, req := range request.RaftShard {
		replica[req.ShardID] = entity.RaftShardConfig{
			BootstrapHost: bootstrapHost.Host, // server overwrite
			ShardID:       shardID,
			ID:            req.ID,
			Type:          req.Type,
			Description:   req.Description,
		}
	}

	return replica, nil
}

func (m *raftApp) getDeploymentTarget(previous *entity.DeploymentJob, request entity.SubmitDeploymentJobRequest) ([]entity.Host, error) {
	// if there is previous deployment, we use that.
	// right now, we do not support other logic.. (can be later in upscale / downscale / change membership scenario)
	if previous != nil {
		return previous.Target, nil
	}

	// else, a new deployment
	var deploymentTarget []entity.Host
	if len(request.TargetHosts) == 0 {
		return nil, errors.New("for new deployment, please specify target host")
	}

	var targetHostErr error
	for _, target := range request.TargetHosts {
		// TODO: validate target
		err := target.Validate()
		if err != nil {
			targetHostErr = errors.Join(targetHostErr, err)
		}

		deploymentTarget = append(deploymentTarget, target)
	}
	if targetHostErr != nil {
		return nil, fmt.Errorf("error determining target host: %w", targetHostErr)
	}

	return deploymentTarget, nil
}

func parseAs[T any](payload []byte) (T, error) {
	var t T
	err := json.Unmarshal(payload, &t)
	return t, err
}

func initDeploymentJobState(job *entity.DeploymentJob) {
	job.Status = entity.DeploymentJobStatusQueued

	job.RestartServiceJob.HostOrder = make([]string, len(job.Target))
	job.RestartServiceJob.Status = make(map[string]entity.HostRestartServiceState, len(job.Target))
	job.ConfigureHostJob.Status = make(map[string]entity.HostConfigurationState, len(job.Target))
	for idx, target := range job.Target {
		job.RestartServiceJob.HostOrder[idx] = target.Host
		job.RestartServiceJob.Status[target.Host] = entity.HostRestartServiceState{
			Status: entity.HostRestartServiceStatusPending,
		}
		job.ConfigureHostJob.Status[target.Host] = entity.HostConfigurationState{
			Status: entity.HostConfigurationStatusPending,
		}
	}
}

// todo: refac fac
func (m *raftApp) getJobByID(ctx context.Context, namespace, service, id string) (*entity.DeploymentJob, error) {
	// check cache
	kc := jobKey{namespace: namespace, service: service, id: id}
	job, ok := m.jobCache.Get(kc)
	if ok {
		return job, nil
	}

	previousJobs, err := m.jobUsecase.Get(ctx, namespace, []string{service}, id)
	if err != nil {
		return nil, err
	}

	_ = m.jobCache.Add(kc, previousJobs[0])

	// if not found it will be err also, so this is safe
	return previousJobs[0], nil
}

func randomPort() uint16 {
	return uint16(rand.Intn(65535-10000) + 10000)
}
