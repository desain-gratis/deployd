package deployjob

import (
	"context"
	"encoding/json"

	raft_runner "github.com/desain-gratis/common/lib/raft/runner"
	"github.com/desain-gratis/deployd/src/entity"
)

var (
// Err..
)

type SubmitJobResponse entity.DeploymentJob
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

	var result entity.DeploymentJob
	err = json.Unmarshal(raftResult, &result)
	if err != nil {
		return SubmitJobResponse{}, err
	}

	return SubmitJobResponse(result), nil
}

func (c *Client) CancelJob(ctx context.Context, request entity.CancelJobRequest) (CancelJobResponse, error) {
	raftResult, value, err := c.Publish(ctx, CommandUserCancelJob, request)
	if err != nil {
		_ = value // can parse error based on value
		return CancelJobResponse{}, err
	}

	var result entity.DeploymentJob
	err = json.Unmarshal(raftResult, &result)
	if err != nil {
		return CancelJobResponse{}, err
	}

	return CancelJobResponse(result), nil
}

func (c *Client) FeedHostConfigurationUpdate(ctx context.Context, request ConfigurationUpdateRequest) error {
	_, value, err := c.Publish(ctx, CommandHostConfigurationUpdate, request)
	if err != nil {
		_ = value
		return err
	}

	return nil
}

func (c *Client) ConfirmDeployment(ctx context.Context, request DeployConfirmation) error {
	_, value, err := c.Publish(ctx, CommandUserDeployConfirmation, request)
	if err != nil {
		_ = value
		return err
	}

	return nil
}

func (c *Client) FeedDeploymentUpdate(ctx context.Context, request DeploymentUpdateRequest) error {
	_, value, err := c.Publish(ctx, CommandHostConfigurationUpdate, request)
	if err != nil {
		_ = value
		return err
	}

	return nil
}
