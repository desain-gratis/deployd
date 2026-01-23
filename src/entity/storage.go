package entity

import (
	"encoding/json"
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
	"github.com/desain-gratis/common/types/entity"
)

var _ mycontent.Data = &BuildArtifact{}

type BuildArtifact struct {
	Id           string          `json:"id,omitempty"`
	Ns           string          `json:"namespace" ch:"namespace"`
	Name         string          `json:"name" ch:"name"`
	CommitID     string          `json:"commit_id" ch:"commit_id"`
	Branch       string          `json:"branch,omitempty" ch:"branch"`
	Actor        string          `json:"actor" ch:"actor"`
	Tag          string          `json:"tag,omitempty" ch:"tag"`
	Data         json.RawMessage `json:"data" ch:"data"`
	PublishedAt  time.Time       `json:"published_at" ch:"published_at"`
	Source       string          `json:"source" ch:"source"`
	RepositoryID string          `json:"repository_id"`
	URLx         string          `json:"url"`
	OsArch       []string        `json:"os_arch" ch:"os_arch"`
	Archive      []*entity.File  `json:"archive"`
}

func (a *BuildArtifact) CreatedTime() time.Time {
	return a.PublishedAt
}

func (a *BuildArtifact) ID() string {
	return a.CommitID
}

func (a *BuildArtifact) Namespace() string {
	return a.Ns
}

func (a *BuildArtifact) RefIDs() []string {
	return []string{a.RepositoryID}
}

func (a *BuildArtifact) URL() string {
	return a.URLx
}

func (a *BuildArtifact) Validate() error {
	return nil
}

func (a *BuildArtifact) WithCreatedTime(t time.Time) mycontent.Data {
	a.PublishedAt = t
	return a
}

func (a *BuildArtifact) WithID(id string) mycontent.Data {
	a.CommitID = id
	return a
}

func (a *BuildArtifact) WithNamespace(id string) mycontent.Data {
	a.Ns = id
	return a
}

func (a *BuildArtifact) WithURL(url string) mycontent.Data {
	a.URLx = url
	return a
}
