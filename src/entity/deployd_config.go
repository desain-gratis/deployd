package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

var _ mycontent.Data = &Host{}

// Host represents raft host configuration for a particular service
type Host struct {
	Ns   string `json:"namespace"`
	Host string `json:"host"`

	Architecture string            `json:"architecture"`
	OS           string            `json:"os"`
	RaftConfig   DeploydRaftConfig `json:"raft_config"`
	FQDN         string            `json:"fqdn"`

	PublishedAt time.Time `json:"published_at" ch:"published_at"`
	URLx        string    `json:"url"`
}

type DeploydRaftConfig struct {
	// (Preferred) replica ID for this host
	ReplicaID uint64 `json:"replica_id"`

	WALDir      string `json:"wal_dir"` // validate / normalize dir
	NodeHostDir string `json:"node_host_dir"`
}

func (a *Host) CreatedTime() time.Time {
	return a.PublishedAt
}

func (a *Host) ID() string {
	return a.Host
}

func (a *Host) Namespace() string {
	return a.Ns
}

func (a *Host) RefIDs() []string {
	return nil
}

func (a *Host) URL() string {
	return a.URLx
}

func (a *Host) Validate() error {
	// TODO: validate raw json
	// a.Replica[x].Config
	return nil
}

func (a *Host) WithCreatedTime(t time.Time) mycontent.Data {
	a.PublishedAt = t
	return a
}

func (a *Host) WithID(id string) mycontent.Data {
	a.Host = id
	return a
}

func (a *Host) WithNamespace(id string) mycontent.Data {
	a.Ns = id
	return a
}

func (a *Host) WithURL(url string) mycontent.Data {
	a.URLx = url
	return a
}
