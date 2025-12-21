package artifactd

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type handler struct {
	conn driver.Conn
}

// A datawriter app / log app, the core logic is in the query; not on any higher-level abstraction

// Common logic for the data writer
func New(conn driver.Conn) *handler {
	return &handler{
		conn: conn,
	}
}

func (h *handler) RegisterCommit(ctx context.Context, data CommitTriggerData) error {
	err := h.conn.Exec(ctx, dmlRegisterCommit,
		data.Namespace, data.Name, data.CommitID, data.Branch, data.Tag, data.Branch, data.Actor, time.Now())
	if err != nil {
		return err
	}

	return nil
}

func (h *handler) GetLatestCommit(ctx context.Context, namespace, name string, duration time.Duration) (<-chan *Commit, error) {
	rows, err := h.conn.Query(ctx, dqlGetCommit, namespace, name, time.Now().Add(-duration))
	if err != nil {
		return nil, err
	}

	out := make(chan *Commit)

	go func() {
		defer close(out)
		defer rows.Close()

		for rows.Next() {
			var commit Commit
			// namespace, name, commit_id, branch, tag, actor, data, created_at
			err := rows.ScanStruct(&commit)
			if err != nil {
				continue
			}
			out <- &commit
		}
	}()

	return out, nil
}
