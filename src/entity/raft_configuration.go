package entity

import (
	"encoding/json"
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

var _ mycontent.Data = &RaftConfiguration{}

type RaftConfiguration struct {
	Id          string                   `json:"id,omitempty"`
	Ns          string                   `json:"namespace" ch:"namespace"`
	Replica     map[uint64]ReplicaConfig `json:"replica"`
	PublishedAt time.Time                `json:"published_at" ch:"published_at"`
	URLx        string                   `json:"url"`
}

type ReplicaConfig struct {
	ReplicaID uint64          `json:"replica_id"`
	Id        string          `json:"id"`
	Alias     string          `json:"alias"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
}

func (a *RaftConfiguration) CreatedTime() time.Time {
	return a.PublishedAt
}

func (a *RaftConfiguration) ID() string {
	return a.Id
}

func (a *RaftConfiguration) Namespace() string {
	return a.Ns
}

func (a *RaftConfiguration) RefIDs() []string {
	return []string{a.Id}
}

func (a *RaftConfiguration) URL() string {
	return a.URLx
}

func (a *RaftConfiguration) Validate() error {
	// TODO: validate raw json
	// a.Replica[x].Config
	return nil
}

func (a *RaftConfiguration) WithCreatedTime(t time.Time) mycontent.Data {
	a.PublishedAt = t
	return a
}

func (a *RaftConfiguration) WithID(id string) mycontent.Data {
	a.Id = id
	return a
}

func (a *RaftConfiguration) WithNamespace(id string) mycontent.Data {
	a.Ns = id
	return a
}

func (a *RaftConfiguration) WithURL(url string) mycontent.Data {
	a.URLx = url
	return a
}
