package configurewebsite

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

type UpdateNginxUnitConfigResponse struct {
	SubmitJobStatus UpdateNginxUnitConfigStatus `json:"submit_job_status,omitempty"` // Ephemeral field, only populated after job reply
	Job             entity.DeploymentJob        `json:"job"`
}

type EventJobSubmitted ConfigureWebsiteResponse
type ConfigureWebsiteResponse struct {
	SubmitJobStatus UpdateNginxUnitConfigStatus `json:"status"`
	Job             *entity.JobConfigureWebsite `json:"job"`
}

type UpdateNginxUnitConfigStatus string

const (
	SubmitJobStatusNeedRetry UpdateNginxUnitConfigStatus = "NEED_RETRY"
	SubmitJobStatusSuccess   UpdateNginxUnitConfigStatus = "SUCCESS"
)

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

func (c *Client) ConfigureWebsite(ctx context.Context, request entity.ConfigureWebsiteRequest) (ConfigureWebsiteResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return ConfigureWebsiteResponse{}, err
	}

	cmdWrap := CommandWrapper{Name: Command_User_ConfigureWebsite, Value: payload}

	proposeResult, err := c.etcdRaftCtx.Propose(ctx, cmdWrap)
	if err != nil {
		return ConfigureWebsiteResponse{}, err
	}

	result, ok := proposeResult.(ConfigureWebsiteResponse)
	if !ok {
		return ConfigureWebsiteResponse{}, fmt.Errorf("unexpeted result type from state machine")
	}

	return result, nil
}

// func (c *Client) UpdateNginxUnitConfig(ctx context.Context, request any) (UpdateNginxUnitConfigResponse, error) {
// 	payload, err := json.Marshal(request)
// 	if err != nil {
// 		return UpdateNginxUnitConfigResponse{}, err
// 	}

// 	cmdWrap := CommandWrapper{Name: Command_User_RequestUpdateNginxUnitConfig, Value: payload}

// 	proposeResult, err := c.etcdRaftCtx.Propose(ctx, cmdWrap)
// 	if err != nil {
// 		return UpdateNginxUnitConfigResponse{}, err
// 	}

// 	result, ok := proposeResult.(UpdateNginxUnitConfigResponse)
// 	if !ok {
// 		return UpdateNginxUnitConfigResponse{}, fmt.Errorf("unexpeted result type from state machine")
// 	}

// 	return result, nil
// }

func (c *Client) GenericUpdate(ctx context.Context, cmd Command, request any) (any, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return UpdateNginxUnitConfigResponse{}, err
	}

	cmdWrap := CommandWrapper{Name: cmd, Value: payload}

	result, err := c.etcdRaftCtx.Propose(ctx, cmdWrap)
	if err != nil {
		return UpdateNginxUnitConfigResponse{}, err
	}

	return result, nil
}
