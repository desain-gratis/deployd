package artifactd

import (
	"context"
	"time"
)

type raftHandler struct {
	*handler
}

func Raft(handler *handler) *raftHandler {
	return &raftHandler{
		handler: handler,
	}
}

func (h *raftHandler) RegisterCommit(ctx context.Context, data CommitTriggerData) error {
	err := h.conn.Exec(ctx, dmlRegisterCommit,
		data.Namespace, data.Name, data.CommitID, data.Branch, data.Tag, data.Branch, data.Actor, time.Now())
	if err != nil {
		return err
	}

	return nil
}
