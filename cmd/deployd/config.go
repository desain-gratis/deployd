package main

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
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

	config.SetEnvPrefix("DEPLOYD")
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
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

func (a *appConfig) getClickhouseConn(name string) (driver.Conn, error) {
	var conn driver.Conn

	func() {
		connslock.RLock()
		defer connslock.RUnlock()

		if conns[name] != nil {
			conn = conns[name]
		}
	}()

	if conn != nil {
		return conn, nil
	}

	cfg := config.Sub("storage").
		Sub("clickhouse").
		Sub(name)

	if cfg == nil {
		log.Fatal().Msgf("empty storageclickhouse config for '%v'", name)
	}

	log.Info().Msgf("name: %v", name)

	cfg.SetDefault("read_timeout", 10*time.Second)

	opts := &clickhouse.Options{
		Addr: cfg.GetStringSlice("address"),
		Auth: clickhouse.Auth{
			Username: cfg.GetString("username"),
			Password: cfg.GetString("password"),
			Database: cfg.GetString("database"),
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		ReadTimeout: cfg.GetDuration("read_timeout"),
	}

	var err error
	err = retry(func() error {
		conn, err = clickhouse.Open(opts)
		return err
	}, 3)

	ctx := context.Background()
	err = retry(func() error {
		return conn.Ping(ctx)
	}, 3)

	func() {
		connslock.Lock()
		defer connslock.Unlock()
		conns[name] = conn
	}()

	return conn, nil
}

func retry(fn func() error, times int) error {
	var attempt int

	var err error
	for {
		attempt++
		err = fn()
		if err == nil || attempt >= times {
			break
		}
		time.Sleep(1 * time.Second)
	}

	return err
}
