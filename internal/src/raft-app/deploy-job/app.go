package deployjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	content_chraft "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/clickhouse-raft"
	"github.com/desain-gratis/common/lib/notifier"
	"github.com/desain-gratis/common/lib/raft"
	"github.com/rs/zerolog/log"

	"github.com/desain-gratis/deployd/src/entity"
)

const (
	TableDeploymentJob = "deployment_job"

	CommandUserSubmitJob           raft.Command = "deployd.user.submit-job"
	CommandUserCancelJob           raft.Command = "deployd.user.cancel-job"
	CommandHostConfigurationUpdate raft.Command = "deployd.host.configuration-update"
	CommandUserDeployConfirmation  raft.Command = "deployd.user.deploy-confirmation"
	CommandHostDeploymentUpdate    raft.Command = "deployd.host.deployment-update"
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

	jobUsecase *mycontent_base.Handler[*entity.DeploymentJob]
}

func New(topic notifier.Topic) *raftApp {
	stateStore := content_chraft.New(
		topic,
		content_chraft.TableConfig{Name: TableDeploymentJob, RefSize: 1, IncrementalID: true, IncrementalIDGetLimit: 10},
	)

	jobStorage, err := stateStore.GetStorage(TableDeploymentJob)
	if err != nil {
		log.Fatal().Msgf("err: %v", err)
	}

	// data accessor inside raft
	jobUsecase := mycontent_base.New[*entity.DeploymentJob](jobStorage, 1)

	return &raftApp{
		topic:      topic,
		ContentApp: stateStore,
		jobUsecase: jobUsecase,
	}
}

// make it easier for everyone..
func (m *raftApp) OnUpdate(ctx context.Context, e raft.Entry) (raft.OnAfterApply, error) {
	switch e.Command {
	case CommandUserSubmitJob:
		payload, err := parseAs[entity.SubmitDeploymentJobRequest](e.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
		}
		return m.userSubmitJob(ctx, payload)
	case CommandUserCancelJob:
		payload, err := parseAs[CancelJobRequest](e.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
		}
		return m.userCancelJob(ctx, payload)
	case CommandHostConfigurationUpdate: // feed installation (sub)state update until finish
		payload, err := parseAs[ConfigurationUpdateRequest](e.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
		}
		return m.hostConfigurationUpdate(ctx, payload)
	case CommandUserDeployConfirmation: // if not "believe", ask user to confirm deployment
		payload, err := parseAs[DeployConfirmation](e.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
		}
		return m.userDeployConfirmation(ctx, payload)
	case CommandHostDeploymentUpdate: // feed deployment update (sub)state update until clean up finish
		payload, err := parseAs[DeploymentUpdateRequest](e.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(e.Value))
		}
		return m.hostDeploymentUpdate(ctx, payload)
	}

	// fallback to the base
	return m.ContentApp.OnUpdate(ctx, e)
}

// Because we're using Golang composition / aka inheritance, we do not need to implement the rest of raft.Application method.
// Later if we have multiple ContentApp, then you need to implement it to make sure all method are executed.

func (m *raftApp) userSubmitJob(ctx context.Context, request entity.SubmitDeploymentJobRequest) (raft.OnAfterApply, error) {
	// needto validate duplication outside raft (in http integration layer)

	// we create the job
	job := &entity.DeploymentJob{
		Ns:          request.Ns,
		Status:      entity.DeploymentJobStatusQueued,
		Request:     request,
		PublishedAt: request.PublishedAt,
	}

	// TODO: utilize metamaxxing
	jobMeta := map[string]any{"author": "kmg"}

	result, err := m.jobUsecase.Post(ctx, job, jobMeta)
	if err != nil {
		return nil, err
	}

	// do something with result (eg. add validation token etc to make sure only user can update, etc.)
	// or we can leave it as it is, depends on the usecase.

	encResult, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	cpResult := *result
	return func() (raft.Result, error) {
		// central event
		m.topic.Broadcast(context.Background(), EventDeploymentJobCreated{Job: cpResult})
		return raft.Result{Value: 0, Data: encResult}, nil
	}, nil
}

func (m *raftApp) userCancelJob(ctx context.Context, request CancelJobRequest) (raft.OnAfterApply, error) {
	previousJobs, err := m.jobUsecase.Get(ctx, request.Ns, []string{request.Service}, request.Id)
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
	case entity.DeploymentJobStatusFinished:
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

func (m *raftApp) hostConfigurationUpdate(ctx context.Context, request ConfigurationUpdateRequest) (raft.OnAfterApply, error) {
	jobs, err := m.jobUsecase.Get(ctx, request.Ns, []string{request.Service}, request.Id)
	if err != nil {
		return nil, err
	}
	job := jobs[0]
	job.Status = request.Status

	job, err = m.jobUsecase.Post(ctx, job, nil)
	if err != nil {
		return nil, err
	}

	encResult, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}

	return func() (raft.Result, error) { return raft.Result{Data: encResult}, nil }, nil
}

func (m *raftApp) userDeployConfirmation(ctx context.Context, request DeployConfirmation) (raft.OnAfterApply, error) {
	return func() (raft.Result, error) { return raft.Result{}, nil }, errors.New("not implemented")
}

func (m *raftApp) hostDeploymentUpdate(ctx context.Context, request DeploymentUpdateRequest) (raft.OnAfterApply, error) {
	return func() (raft.Result, error) { return raft.Result{}, nil }, errors.New("not implemented")
}

func parseAs[T any](payload []byte) (T, error) {
	var t T
	err := json.Unmarshal(payload, &t)
	return t, err
}
