package configurewebsite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	content_badger "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/badger"
	"github.com/desain-gratis/common/lib/notifier"
	"github.com/desain-gratis/common/lib/raft"
	"github.com/dgraph-io/badger/v4"
	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/desain-gratis/deployd/src/entity"
)

// static
const (
	namespace = "deployd"
	name      = "configure-website"
)

type Command string
type Event string

const (
	TableConfigureWebsiteJob = "configure_website"
	TableDeploymentSuccess   = "deploy_job__deployment_success"

	Command_User_RequestUpdateNginxUnitConfig Command = "user.request-update-ngingx-unit-config"
	Command_Host_ConfigureWebsiteResponse     Command = "host.configure-website-result"
	Command_User_ConfigureWebsite             Command = "user.configure-website"

	Event_AllHostConfigured Event = "app.all-host-configured"
)

var _ raft.ApplicationV2 = (*RaftApp)(nil)

type RaftApp struct {
	topic notifier.Topic

	jobCache   *expirable.LRU[jobKey, *entity.JobConfigureWebsite]
	jobUsecase *mycontent_base.Handler[*entity.JobConfigureWebsite]
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

var ErrRetryable = errors.New("retryable")

func New(topic notifier.Topic, dbJob *badger.DB) *RaftApp {
	jobStorage := content_badger.NewAutoIncrement(dbJob, TableConfigureWebsiteJob, 1)
	jobUsecase := mycontent_base.New[*entity.JobConfigureWebsite](jobStorage)
	jobCache := expirable.NewLRU[jobKey, *entity.JobConfigureWebsite](256, nil, 20*time.Minute) // at least until the DB can catch up

	return &RaftApp{
		topic:      topic,
		jobUsecase: jobUsecase,
		jobCache:   jobCache,
	}
}

func (m *RaftApp) GetJobStore() *mycontent_base.Handler[*entity.JobConfigureWebsite] {
	return m.jobUsecase
}

func (m *RaftApp) OnUpdateV2(ctx context.Context, entry raft.EntryV2) (any, error) {
	cmd, err := parseAs[CommandWrapper](entry.Data)
	if err != nil {
		return nil, err
	}

	switch cmd.Name {
	case Command_User_ConfigureWebsite:
		// start create job
		payload, err := parseAs[entity.ConfigureWebsiteRequest](cmd.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(cmd.Value))
		}
		result, err := m.userSubmitJob(ctx, payload)
		if err != nil {
			return nil, fmt.Errorf("failed to submit job: %w", err)
		}
		return result()
	case Command_Host_ConfigureWebsiteResponse:
		payload, err := parseAs[entity.HostUpdateResult](cmd.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse command as JSON (%v)", err, string(cmd.Value))
		}
		result, err := m.hostResponse(ctx, payload)
		if err != nil {
			return nil, fmt.Errorf("failed to submit job: %w", err)
		}
		return result()
	default:
		return nil, fmt.Errorf("unknown command: %s", cmd.Name)
	}

}

// Because we're using Golang composition / aka inheritance, we do not need to implement the rest of raft.Application method.
// Later if we have multiple ContentApp, then you need to implement it to make sure all method are executed.
func (m *RaftApp) userSubmitJob(ctx context.Context, request entity.ConfigureWebsiteRequest) (ApplyResult, error) {
	newJob, err := m.jobUsecase.Post(ctx, &entity.JobConfigureWebsite{
		Ns:          namespace,
		Name:        name,
		Request:     request,
		Status:      "QUEUED", // queued inside the raft :)
		PublishedAt: request.Time,
	}, nil)
	if err != nil {
		return nil, err
	}

	resp := ConfigureWebsiteResponse{SubmitJobStatus: SubmitJobStatusSuccess, Job: newJob}

	return func() (any, error) {
		m.topic.Broadcast(ctx, EventJobSubmitted(resp))
		return resp, nil
	}, nil
}

func (m *RaftApp) hostResponse(ctx context.Context, request entity.HostUpdateResult) (ApplyResult, error) {
	currentJob, err := m.getJobByID(ctx, namespace, []string{name}, request.JobID)
	if err != nil {
		return nil, err
	}

	if currentJob.Result == nil {
		currentJob.Result = make(map[string]*entity.HostUpdateResult)
	}

	currentJob.Result[request.HostID] = &request

	// TODO: determine job successful state here if all host successfully updated
	var finished int
	var successfulHostResponse int
	for _, result := range currentJob.Result {
		if result.Status == entity.JobConfigureWebsiteStatusSuccess {
			successfulHostResponse++
		}
		if result.Status.IsTerminal() {
			finished++
		}
	}

	var isFinished bool
	var isSuccessful bool

	if finished == len(currentJob.Request.TargetHosts) {
		isFinished = true
		if successfulHostResponse == finished {
			currentJob.Status = "ZUCCESS"
			isSuccessful = true
			currentJob.FinishedAt = request.Time
		} else {
			currentJob.Status = "FAILED"
		}
	}

	_, err = m.jobUsecase.Post(ctx, currentJob, nil)
	if err != nil {
		return nil, err
	}

	return func() (any, error) {
		if isFinished && isSuccessful {
			m.topic.Broadcast(ctx, "job completed successfully")
		} else if isFinished {
			m.topic.Broadcast(ctx, "job failed")
		}
		return request, nil
	}, nil
}

func parseAs[T any](payload []byte) (T, error) {
	var t T
	err := json.Unmarshal(payload, &t)
	return t, err
}

// todo: refac fac
func (m *RaftApp) getJobByID(ctx context.Context, namespace string, refIDs []string, id string) (*entity.JobConfigureWebsite, error) {
	// TODO~~
	kc := jobKey{namespace: namespace, service: strings.Join(refIDs, "\\"), id: id}
	job, ok := m.jobCache.Get(kc)
	if ok {
		return job, nil
	}

	previousJobs, err := m.jobUsecase.Get(ctx, namespace, refIDs, id)
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
