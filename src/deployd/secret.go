package deployd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

const (
	envSecretConfigPath = "DEPLOYD_SECRET"
)

var (
	ErrNotConfigured = errors.New("not configured")
)

// InjectSecretToViper inject secret from the application secret path
func InjectSecretToViper(v *viper.Viper) error {
	secretConfigFile := os.Getenv(envSecretConfigPath)

	if secretConfigFile == "" {
		return ErrNotConfigured
	}

	config := viper.New()
	config.SetConfigType("yaml")

	f, err := os.Open(secretConfigFile)
	if err != nil {
		return fmt.Errorf("deployd open secret file: %w", err)
	}

	err = config.ReadConfig(f)
	if err != nil {
		return fmt.Errorf("deployd read secret file: %w", err)
	}

	return v.MergeConfigMap(config.AllSettings())
}
