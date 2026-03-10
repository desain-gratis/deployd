package hello

import (
	"context"
	"time"

	"github.com/desain-gratis/common/lib/raft/runner"
	raft_runner "github.com/desain-gratis/common/lib/raft/runner"
	"github.com/desain-gratis/deployd/src/entity"
	"github.com/lni/dragonboat/v4/client"
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
	session *client.Session
	rc      raft_runner.RaftContext
}

func NewClient(ctx context.Context) *Client {
	// client for "worker" / "local integration" to communicate with Raft app
	rClient, err := runner.NewClient(ctx)
	if err != nil {
		log.Fatal().Msgf("err: %v", err)
	}

	raftCtx, _ := runner.GetRaftContext(ctx)

	return &Client{
		Client:  rClient,
		rc:      raftCtx,
		session: raftCtx.DHost.GetNoOPSession(raftCtx.ShardID),
	}
}

func (c *Client) GetGreeting(ctx context.Context) (string, error) {
	// raftResult, value, err := c.Publish(ctx, "get-greetings", "hello")
	// if err != nil {
	// 	_ = value // can parse error based on value
	// 	return "", err
	// }

	// Try sync propose base
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	raftRes, err := c.rc.DHost.SyncPropose(ctx, c.session, []byte(`{"msg":"get-greetings:hello"}`))
	if err != nil {
		return "", err
	}
	raftResult := raftRes.Data

	// Try (async) propose base, try also with commited
	// c.rc.DHost.Propose(c.session, "get-greetings:hello", 1*time.Minute)

	return string(raftResult), nil
}
