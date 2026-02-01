package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

var _ mycontent.Data = &RaftHost{}

// RaftHost represents raft host configuration for a particular service
type RaftHost struct {
	Id        string `json:"id,omitempty"`
	Ns        string `json:"namespace" ch:"namespace"`
	ServiceID string `json:"service_id"`

	ReplicaID      uint64 `json:"replica_id"`
	RaftAdress     string `json:"raft_address"`
	WALDir         string `json:"wal_dir"`
	NodeHostDir    string `json:"node_host_dir"`
	RTTMillisecond uint64 `json:"rtt_millisecond"`
	DeploymentID   uint64 `json:"deployment_id"`

	PublishedAt time.Time `json:"published_at" ch:"published_at"`
	URLx        string    `json:"url"`
}

type ClusterConfiguration struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	RaftDir string `json:"raft_dir"`
}

type StorageConfiguration struct {
	Clickhouse ClickHouseStorageConfig `json:"clickhouse"`
}

type ClickHouseStorageConfig struct {
	Address string `json:"address"`
}

func (a *RaftHost) CreatedTime() time.Time {
	return a.PublishedAt
}

func (a *RaftHost) ID() string {
	return a.Id
}

func (a *RaftHost) Namespace() string {
	return a.Ns
}

func (a *RaftHost) RefIDs() []string {
	return []string{a.Id}
}

func (a *RaftHost) URL() string {
	return a.URLx
}

func (a *RaftHost) Validate() error {
	// TODO: validate raw json
	// a.Replica[x].Config
	return nil
}

func (a *RaftHost) WithCreatedTime(t time.Time) mycontent.Data {
	a.PublishedAt = t
	return a
}

func (a *RaftHost) WithID(id string) mycontent.Data {
	a.Id = id
	return a
}

func (a *RaftHost) WithNamespace(id string) mycontent.Data {
	a.Ns = id
	return a
}

func (a *RaftHost) WithURL(url string) mycontent.Data {
	a.URLx = url
	return a
}
