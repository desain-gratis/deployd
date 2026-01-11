package artifactd

import (
	"encoding/json"
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

var _ mycontent.Data = &Artifact{}

type Artifact struct {
	IDx          uint32          `json:"id" ch:"id"`
	UID          string          `json:"uid"`
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
}

func (a *Artifact) CreatedTime() time.Time {
	return a.PublishedAt
}

func (a *Artifact) ID() string {
	return a.UID
}

func (a *Artifact) Namespace() string {
	return a.Ns
}

func (a *Artifact) RefIDs() []string {
	return []string{a.RepositoryID}
}

func (a *Artifact) URL() string {
	return a.URLx
}

func (a *Artifact) Validate() error {
	return nil
}

func (a *Artifact) WithCreatedTime(t time.Time) mycontent.Data {
	a.PublishedAt = t
	return a
}

func (a *Artifact) WithID(id string) mycontent.Data {
	a.UID = id
	return a
}

func (a *Artifact) WithNamespace(id string) mycontent.Data {
	a.Ns = id
	return a
}

func (a *Artifact) WithURL(url string) mycontent.Data {
	a.URLx = url
	return a
}
