package artifactd

import (
	"encoding/json"
	"time"
)

type Command struct {
	Command string          `json:"command"`
	Data    json.RawMessage `json:"data"`
}

type PublishCommitRequest struct {
}

type QueryArtifact struct {
	Namespace string
	Name      string
	From      time.Time
}
