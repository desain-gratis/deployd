package hello

import (
	"context"

	content_chraft "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/clickhouse-raft"
	"github.com/desain-gratis/common/lib/raft"
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
}

func New() *raftApp {
	stateStore := content_chraft.New() // base state store without table

	// cache is important because we don't rely on DB for get-and-set operation (expect stale data, trade off with high write)
	return &raftApp{
		ContentApp: stateStore,
	}
}

// make it easier for everyone..
func (m *raftApp) OnUpdate(ctx context.Context, e raft.Entry) (raft.OnAfterApply, error) {
	return func() (raft.Result, error) {
		return raft.Result{Value: 0, Data: []byte("Hello, World!")}, nil
	}, nil
}
