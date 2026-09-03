package deployjob

import (
	"context"
	"encoding/json"
	"fmt"

	dgraft "github.com/desain-gratis/common/lib/raft"
	runneretcd "github.com/desain-gratis/common/lib/raft/runner-etcd"
	"github.com/rs/zerolog/log"

	"github.com/desain-gratis/deployd/src/entity"
)

var (
// Err..
)

type SubmitJobResponse struct {
	SubmitJobStatus SubmitJobStatus      `json:"submit_job_status,omitempty"` // Ephemeral field, only populated after job reply
	Job             entity.DeploymentJob `json:"job"`
}

type SubmitJobStatus string

const (
	SubmitJobStatusNeedRetry SubmitJobStatus = "NEED_RETRY"
	SubmitJobStatusSuccess   SubmitJobStatus = "SUCCESS"
)

type CancelJobResponse entity.DeploymentJob

type Client struct {
	// *raft_runner.Client
	etcdRaftCtx *runneretcd.RaftContext
}

func NewClient(ctx context.Context) *Client {
	// client for "worker" / "local integration" to communicate with Raft app

	rCtx, ok := dgraft.GetRaftContext(ctx).(*runneretcd.RaftContext)
	if !ok {
		log.Fatal().Msgf("not an etcd raft runner")
	}

	return &Client{
		etcdRaftCtx: rCtx,
	}
}

func (c *Client) SubmitJob(ctx context.Context, request entity.SubmitDeploymentJobRequest) (SubmitJobResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return SubmitJobResponse{}, err
	}

	cmdWrap := CommandWrapper{Name: CommandUserSubmitJob, Value: payload}

	proposeResult, err := c.etcdRaftCtx.Propose(ctx, cmdWrap)
	if err != nil {
		return SubmitJobResponse{}, err
	}

	raftResult, ok := proposeResult.([]byte)
	if !ok {
		return SubmitJobResponse{}, fmt.Errorf("unexpeted result type from state machine")
	}

	result, err := parseAs[SubmitJobResponse](raftResult)
	if err != nil {
		return SubmitJobResponse{}, err
	}

	return result, nil
}

func (c *Client) CancelJob(ctx context.Context, request entity.CancelJobRequest) (CancelJobResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return CancelJobResponse{}, err
	}

	cmdWrap := CommandWrapper{Name: CommandUserCancelJob, Value: payload}

	proposeResult, err := c.etcdRaftCtx.Propose(ctx, cmdWrap)
	if err != nil {
		return CancelJobResponse{}, err
	}

	raftResult, ok := proposeResult.([]byte)
	if !ok {
		return CancelJobResponse{}, fmt.Errorf("unexpeted result type from state machine")
	}

	result, err := parseAs[CancelJobResponse](raftResult)
	if err != nil {
		return CancelJobResponse{}, err
	}

	return result, nil
}

func (c *Client) FeedHostConfigurationUpdate(ctx context.Context, request ConfigurationUpdateRequest) (ConfigurationUpdateResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return ConfigurationUpdateResponse{}, err
	}

	cmdWrap := CommandWrapper{Name: CommandHostConfigurationUpdate, Value: payload}

	proposeResult, err := c.etcdRaftCtx.Propose(ctx, cmdWrap)
	if err != nil {
		return ConfigurationUpdateResponse{}, err
	}

	raftResult, ok := proposeResult.([]byte)
	if !ok {
		return ConfigurationUpdateResponse{}, fmt.Errorf("unexpeted result type from state machine")
	}

	result, err := parseAs[ConfigurationUpdateResponse](raftResult)
	if err != nil {
		return ConfigurationUpdateResponse{}, err
	}

	return result, nil
}

func (c *Client) ConfirmRestartService(ctx context.Context, request RestartConfirmation) (HostRestartConfirmationResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return HostRestartConfirmationResponse{}, err
	}

	cmdWrap := CommandWrapper{Name: CommandRestartConfirmation, Value: payload}

	proposeResult, err := c.etcdRaftCtx.Propose(ctx, cmdWrap)
	if err != nil {
		return HostRestartConfirmationResponse{}, err
	}

	raftResult, ok := proposeResult.([]byte)
	if !ok {
		return HostRestartConfirmationResponse{}, fmt.Errorf("unexpeted result type from state machine")
	}

	result, err := parseAs[HostRestartConfirmationResponse](raftResult)
	if err != nil {
		return HostRestartConfirmationResponse{}, err
	}

	return result, nil
}

func (c *Client) FeedHostRestartServiceUpdate(ctx context.Context, request HostRestartServiceUpdateRequest) (HostRestartServiceUpdateResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return HostRestartServiceUpdateResponse{}, err
	}

	cmdWrap := CommandWrapper{Name: CommandHostRestartServiceUpdate, Value: payload}

	proposeResult, err := c.etcdRaftCtx.Propose(ctx, cmdWrap)
	if err != nil {
		return HostRestartServiceUpdateResponse{}, err
	}

	raftResult, ok := proposeResult.([]byte)
	if !ok {
		return HostRestartServiceUpdateResponse{}, fmt.Errorf("unexpeted result type from state machine")
	}

	result, err := parseAs[HostRestartServiceUpdateResponse](raftResult)
	if err != nil {
		return HostRestartServiceUpdateResponse{}, err
	}

	return result, nil
}
