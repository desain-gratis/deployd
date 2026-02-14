package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

var _ mycontent.Data = &ServiceInstanceHost{}

// RaftHost represents raft host configuration for a particular service
type ServiceInstanceHost struct {
	Ns      string `json:"namespace"`
	Service string `json:"service"` // service
	Host    string `json:"host"`    // where is this deployed, the ID

	RaftConfig *RaftConfig `json:"raft_config,omitempty"`

	// current on progress deployment (if any)
	ActiveDeployment *HostDeploymentJob `json:"active_deployment,omitempty"`
	LatestDeployment *HostDeploymentJob `json:"latest_deployment,omitempty"`

	PublishedAt time.Time `json:"published_at"`
	URLx        string    `json:"url"`
}

type RaftConfig struct {
	RaftPort       uint16 `json:"raft_port"`
	ReplicaID      uint64 `json:"replica_id"`
	RaftWALDir     string `json:"wal_dir"`
	NodeHostDir    string `json:"node_host_dir"`
	RTTMillisecond uint64 `json:"rtt_millisecond"`
	DeploymentID   uint64 `json:"deployment_id"`
}

func (a *ServiceInstanceHost) CreatedTime() time.Time {
	return a.PublishedAt
}

func (a *ServiceInstanceHost) ID() string {
	return a.Host
}

func (a *ServiceInstanceHost) Namespace() string {
	return a.Ns
}

func (a *ServiceInstanceHost) RefIDs() []string {
	return []string{a.Service}
}

func (a *ServiceInstanceHost) URL() string {
	return a.URLx
}

func (a *ServiceInstanceHost) Validate() error {
	// TODO: validate raw json
	// a.Replica[x].Config
	return nil
}

func (a *ServiceInstanceHost) WithCreatedTime(t time.Time) mycontent.Data {
	a.PublishedAt = t
	return a
}

func (a *ServiceInstanceHost) WithID(id string) mycontent.Data {
	a.Host = id
	return a
}

func (a *ServiceInstanceHost) WithNamespace(id string) mycontent.Data {
	a.Ns = id
	return a
}

func (a *ServiceInstanceHost) WithURL(url string) mycontent.Data {
	a.URLx = url
	return a
}
