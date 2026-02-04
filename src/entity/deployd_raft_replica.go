package entity

import (
	"encoding/json"
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

var _ mycontent.Data = &RaftReplica{}

// RaftReplica represents raft replica configuration for a particular service
type RaftReplica struct {
	Id        string `json:"id,omitempty"`
	Ns        string `json:"namespace" ch:"namespace"`
	ServiceID string `json:"service_id"`

	ReplicaConfig map[uint64]ReplicaConfig `json:"replica_config"`

	PublishedAt time.Time `json:"published_at" ch:"published_at"`
	URLx        string    `json:"url"`
}

type ReplicaConfig struct {
	ID          string          `json:"id"`
	ReplicaID   uint64          `json:"replica_id"`
	Alias       string          `json:"alias"`
	Description string          `json:"description"`
	Type        string          `json:"type"`
	Config      json.RawMessage `json:"config"`
}

func (a *RaftReplica) CreatedTime() time.Time {
	return a.PublishedAt
}

func (a *RaftReplica) ID() string {
	return a.Id
}

func (a *RaftReplica) Namespace() string {
	return a.Ns
}

func (a *RaftReplica) RefIDs() []string {
	return []string{a.Id}
}

func (a *RaftReplica) URL() string {
	return a.URLx
}

func (a *RaftReplica) Validate() error {
	// TODO: validate raw json
	// a.Replica[x].Config
	return nil
}

func (a *RaftReplica) WithCreatedTime(t time.Time) mycontent.Data {
	a.PublishedAt = t
	return a
}

func (a *RaftReplica) WithID(id string) mycontent.Data {
	a.Id = id
	return a
}

func (a *RaftReplica) WithNamespace(id string) mycontent.Data {
	a.Ns = id
	return a
}

func (a *RaftReplica) WithURL(url string) mycontent.Data {
	a.URLx = url
	return a
}
