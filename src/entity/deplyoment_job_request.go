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

	ModifyKey *string `json:"-"` // hidden

	IsBelieve   bool      `json:"is_believe"`
	Url         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}
