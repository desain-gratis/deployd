package hello

import (
	"context"

	"github.com/desain-gratis/common/lib/raft/runner"
	raft_runner "github.com/desain-gratis/common/lib/raft/runner"
	"github.com/desain-gratis/deployd/src/entity"
	"github.com/rs/zerolog/log"
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
	*raft_runner.Client
}

func NewClient(ctx context.Context) *Client {
	// client for "worker" / "local integration" to communicate with Raft app
	rClient, err := runner.NewClient(ctx)
	if err != nil {
		log.Fatal().Msgf("err: %v", err)
	}

	return &Client{
		Client: rClient,
	}
}

func (c *Client) GetGreeting(ctx context.Context) (string, error) {
	raftResult, value, err := c.Publish(ctx, "get-greetings", "hello")
	if err != nil {
		_ = value // can parse error based on value
		return "", err
	}

	return string(raftResult), nil
}
