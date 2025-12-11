package artifactd

import (
	"context"
	"encoding/json"
	"time"
)

type CommitTriggerData struct {
	Namespace   string          `json:"namespace" ch:"namespace"`
	Name        string          `json:"name" ch:"name"`
	CommitID    string          `json:"commit_id" ch:"commit_id"`
	Branch      string          `json:"branch,omitempty" ch:"branch"`
	Actor       string          `json:"actor" ch:"actor"`
	Tag         string          `json:"tag,omitempty" ch:"tag"`
	Data        json.RawMessage `json:"data" ch:"data"`
	PublishedAt time.Time       `json:"published_at" ch:"published_at"`
}

type Commit struct {
	CommitID string `json:"commit_id"`
}

type App interface {
	RegisterCommit(ctx context.Context, data CommitTriggerData) error
	GetLatestCommit(ctx context.Context, duration time.Duration) (<-chan Commit, error)
}
