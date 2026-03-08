package entity

import (
	"time"
)

type CancelJobRequest struct {
	Ns      string `json:"namespace"`
	Id      string `json:"id"`
	Service string `json:"service"`
}

type SubmitDeploymentJobRequest struct {
	Ns      string            `json:"namespace"`
	Service ServiceDefinition `json:"service"`
	Id      string            `json:"id"`

	BuildVersion  uint64 `json:"build_version"`
	SecretVersion uint64 `json:"secret_version"`
	EnvVersion    uint64 `json:"env_version"`

	// optional: lookup configure cloudflare routing
	RoutingVersion *uint64 `json:"routing_version,omitempty"`

	// TODO: List of hosts to deploy to; for first time deployment
	TargetHosts []Host `json:"target_hosts,omitempty"`

	// Register (new) replica at the deployment time.
	RaftReplica      map[uint64]RaftReplicaConfig `json:"raft_replica,omitempty"`
	RaftPort         uint16                       `json:"raft_port"`
	RaftPortMapping  map[string]uint16            `json:"raft_port_mapping,omitempty"` // optional if at each host, the port is different
	RaftDeploymentID uint64                       `json:"raft_deployment_id"`

	ModifyKey *string `json:"-"` // hidden; TODO: to be nice, to lock, only the one who have this key can modify the state.

	// TODO: cloudflared configuration

	// TODO: deployment worker job script name (eg. Ubuntu), in which they will define the DAG, and will spawn a worker instances

	// TODO: After worker is spawned, they may notify their instance name & HTTP address here for anyone who wants to stream its log;
	// for more detailed, worker specific, logs.
	// we do have update in the Raft level (this struct), but it's less detailed.

	TimeoutSeconds *uint32 `json:"timeout_seconds,omitempty"`

	IsBelieve   bool      `json:"is_believe"`
	Url         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

type RaftReplicaConfig struct {
	BootstrapHost string `json:"bootstrap_host"`
	ShardID       uint64 `json:"shard_id"`
	ID            string `json:"id"`
	Type          string `json:"type"`
	Description   string `json:"description"`
}

type RaftServiceConfig struct {
	ReplicaID    uint64 `json:"replica_id"` // convenience from host
	DeploymentID uint64 `json:"deployment_id"`
	RaftAddress  string `json:"raft_address"`
}

type RaftHostConfig struct {
	ReplicaID       uint64 `json:"replica_id"`
	BaseWALDir      string `json:"base_wal_dir"`
	BaseNodeHostDir string `json:"base_node_host_dir"`
	RTTMillisecond  uint64 `json:"rtt_millisecond"`
}
