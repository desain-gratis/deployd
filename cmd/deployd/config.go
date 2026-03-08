package main

import (
	"os"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var (
	conns     = make(map[string]driver.Conn)
	connslock = new(sync.RWMutex)
	config    = &appConfig{viper.New()}
)

type appConfig struct {
	*viper.Viper
}

func initConfig() {
	config.SetConfigType("yaml")

	// config.SetEnvPrefix("DEPLOYD")
	// config.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	config.AutomaticEnv()

	configFile := os.Getenv("CONFIG")

	f, err := os.Open(configFile)
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}

	err = config.ReadConfig(f)
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}
}
