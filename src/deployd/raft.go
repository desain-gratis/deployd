package deployd

import (
	"os"

	raft_replica "github.com/desain-gratis/common/lib/raft/replica"
)

const (
	envRaftConfigPath = "DEPLOYD_RAFT"
)

func InitializeRaft() error {
	raftConfig := os.Getenv(envRaftConfigPath)

	if raftConfig == "" {
		return raft_replica.Init()
	}

	return raft_replica.InitWithConfigFile(raftConfig)
}
