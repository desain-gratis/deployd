package systemd

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/rs/zerolog/log"
)

type handler struct{}

func New(ctx context.Context) {
	go initializeListener(ctx)
}

func initializeListener(ctx context.Context) {
	// Connect to systemd
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		log.Panic().Msgf("Failed to connect to systemd: %v", err)
	}
	defer conn.Close()

	// Print initial service list
	fmt.Println("=== Current Services ===")
	units, err := conn.ListUnitsContext(ctx)
	if err != nil {
		log.Panic().Msgf("Failed to connect to systemd: %v", err)
	}

	for _, u := range units {
		log.Info().Msgf("unit: %v job type: %v description: %v", u.Name, u.JobType, u.Description)
		if u.JobType == "service" {
			fmt.Printf("%-40s %-10s %-10s\n", u.Name, u.ActiveState, u.SubState)
		}
	}

	fmt.Println("\n=== Watching for Service Changes ===")

	// Channel for DBus event notifications
	changes, errChan := conn.SubscribeUnits(0)

	for {
		select {
		case changedUnits := <-changes:
			for name, unit := range changedUnits {
				// Only show services
				if !strings.HasSuffix(unit.Name, ".service") || unit.JobType != "service" {
					continue
				}

				fmt.Printf("[EVENT] %-40s  %-10s  %-10s\n",
					name,
					unit.ActiveState,
					unit.SubState,
				)
			}
		case err := <-errChan:
			log.Err(err).Msgf("error received")
		}
	}
}

// subscribe to systemd's service
func (h *handler) Subscribe() {

}
