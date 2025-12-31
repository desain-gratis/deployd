package artifactd

import (
	"encoding/json"
	"time"
)

type Artifact struct {
	ID          uint32          `json:"id" ch:"id"`
	Namespace   string          `json:"namespace" ch:"namespace"`
	Name        string          `json:"name" ch:"name"`
	CommitID    string          `json:"commit_id" ch:"commit_id"`
	Branch      string          `json:"branch,omitempty" ch:"branch"`
	Actor       string          `json:"actor" ch:"actor"`
	Tag         string          `json:"tag,omitempty" ch:"tag"`
	Data        json.RawMessage `json:"data" ch:"data"`
	PublishedAt time.Time       `json:"published_at" ch:"published_at"`
	Source      string          `json:"source" ch:"source"`
	OsArch      []string        `json:"os_arch" ch:"os_arch"`
}
