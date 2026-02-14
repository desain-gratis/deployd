package deployjob

import (
	"context"

	raft_runner "github.com/desain-gratis/common/lib/raft/runner"
	"github.com/desain-gratis/deployd/src/entity"
)

var (
// Err..
)

type SubmitJobResponse struct {
	SubmitJobStatus SubmitJobStatus `json:"submit_job_status,omitempty"` // Ephemeral field, only populated after job reply
	Job             entity.DeploymentJob
}

type SubmitJobStatus string

const (
	SubmitJobStatusNeedRetry SubmitJobStatus = "NEED_RETRY"
	SubmitJobStatusSuccess   SubmitJobStatus = "SUCCESS"
)

type CancelJobResponse entity.DeploymentJob

type Client struct {
	*raft_runner.Client
}

func NewClient(raftClient *raft_runner.Client) *Client {
	return &Client{
		Client: raftClient,
	}
}

func (c *Client) SubmitJob(ctx context.Context, request entity.SubmitDeploymentJobRequest) (SubmitJobResponse, error) {
	raftResult, value, err := c.Publish(ctx, CommandUserSubmitJob, request)
	if err != nil {
		_ = value // can parse error based on value
		return SubmitJobResponse{}, err
	}

	result, err := parseAs[SubmitJobResponse](raftResult)
	if err != nil {
		return SubmitJobResponse{}, err
	}

	return result, nil
}

func (c *Client) CancelJob(ctx context.Context, request entity.CancelJobRequest) (CancelJobResponse, error) {
	raftResult, value, err := c.Publish(ctx, CommandUserCancelJob, request)
	if err != nil {
		_ = value // can parse error based on value
		return CancelJobResponse{}, err
	}

	result, err := parseAs[CancelJobResponse](raftResult)
	if err != nil {
		return CancelJobResponse{}, err
	}

	return result, nil
}

func (c *Client) FeedHostConfigurationUpdate(ctx context.Context, request ConfigurationUpdateRequest) (ConfigurationUpdateResponse, error) {
	raftResult, value, err := c.Publish(ctx, CommandHostConfigurationUpdate, request)
	if err != nil {
		_ = value
		return ConfigurationUpdateResponse{}, err
	}

	result, err := parseAs[ConfigurationUpdateResponse](raftResult)
	if err != nil {
		return ConfigurationUpdateResponse{}, err
	}

	return result, nil
}

func (c *Client) ConfirmRestartService(ctx context.Context, request RestartConfirmation) (HostRestartConfirmationResponse, error) {
	raftResult, value, err := c.Publish(ctx, CommandRestartConfirmation, request)
	if err != nil {
		_ = value
		return HostRestartConfirmationResponse{}, err
	}

	result, err := parseAs[HostRestartConfirmationResponse](raftResult)
	if err != nil {
		return HostRestartConfirmationResponse{}, err
	}

	return result, nil
}

func (c *Client) FeedHostRestartServiceUpdate(ctx context.Context, request HostRestartServiceUpdateRequest) (HostRestartServiceUpdateResponse, error) {
	raftResult, value, err := c.Publish(ctx, CommandHostRestartServiceUpdate, request)
	if err != nil {
		_ = value
		return HostRestartServiceUpdateResponse{}, err
	}

	result, err := parseAs[HostRestartServiceUpdateResponse](raftResult)
	if err != nil {
		return HostRestartServiceUpdateResponse{}, err
	}

	return result, nil
}
