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
	Id      string            `json:"string"`

	BuildVersion             uint64 `json:"build_version"`
	SecretVersion            uint64 `json:"secret_version"`
	EnvVersion               uint64 `json:"env_version"`
	RaftConfigVersion        uint64 `json:"raft_config_version"`
	RaftConfigReplicaVersion uint64 `json:"raft_config_replica_version"`

	ModifyKey *string `json:"-"` // hidden; TODO: to be nice, to lock, only the one who have this key can modify the state.

	// TODO: List of hosts to deploy to

	// TODO: deployment worker job script name (eg. Ubuntu), in which they will define the DAG, and will spawn a worker instances

	// TODO: After worker is spawned, they may notify their instance name & HTTP address here for anyone who wants to stream its log;
	// for more detailed, worker specific, logs.
	// we do have update in the Raft level (this struct), but it's less detailed.

	TimeoutSeconds *uint32 `json:"timeout_seconds,omitempty"`

	IsBelieve   bool      `json:"is_believe"`
	Url         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}
