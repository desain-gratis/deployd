package deployd

import (
	"os"

	raftr "github.com/desain-gratis/common/lib/raft/runner"
)

func InitRaft() error {
	configFile := os.Getenv(envRaftConfigPath)

	if configFile == "" {
		return ErrNotConfigured
	}

	err := raftr.InitWithConfigFile(configFile)
	if err != nil {
		return err
	}

	return nil
}
