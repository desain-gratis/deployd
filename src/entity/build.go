package entity

import (
	"encoding/json"
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

var _ mycontent.Data = &Build{}

type Build struct {
	IDx         uint32          `json:"id" ch:"id"`
	UID         string          `json:"uid"`
	Ns          string          `json:"namespace" ch:"namespace"`
	Name        string          `json:"name" ch:"name"`
	CommitID    string          `json:"commit_id" ch:"commit_id"`
	Branch      string          `json:"branch,omitempty" ch:"branch"`
	Actor       string          `json:"actor" ch:"actor"`
	Tag         string          `json:"tag,omitempty" ch:"tag"`
	Data        json.RawMessage `json:"data" ch:"data"`
	PublishedAt time.Time       `json:"published_at" ch:"published_at"`
	Source      string          `json:"source" ch:"source"`
	URLx        string          `json:"url"`
	OsArch      []string        `json:"os_arch" ch:"os_arch"`
}

func (b *Build) CreatedTime() time.Time {
	return b.PublishedAt
}

func (b *Build) ID() string {
	return b.UID
}

func (b *Build) Namespace() string {
	return b.Ns
}

func (b *Build) RefIDs() []string {
	return nil
}

func (b *Build) URL() string {
	return b.URLx
}

func (b *Build) Validate() error {
	return nil
}

func (b *Build) WithCreatedTime(t time.Time) mycontent.Data {
	b.PublishedAt = t
	return b
}

func (b *Build) WithID(id string) mycontent.Data {
	b.UID = id
	return b
}

func (b *Build) WithNamespace(id string) mycontent.Data {
	b.Ns = id
	return b
}

func (b *Build) WithURL(url string) mycontent.Data {
	b.URLx = url
	return b
}
