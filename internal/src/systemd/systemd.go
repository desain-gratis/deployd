package systemd

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/desain-gratis/common/lib/notifier"
	"github.com/rs/zerolog/log"
)

type handler struct {
	status   map[string]*DBusUnitStatus
	topic    notifier.Topic
	mu       *sync.RWMutex
	ready    bool
	dbusConn *dbus.Conn
}

func New(ctx context.Context, topic notifier.Topic) *handler {
	h := &handler{
		status: make(map[string]*DBusUnitStatus),
		topic:  topic,
		mu:     &sync.RWMutex{},
	}
	go h.initializeListener(ctx, topic)

	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		log.Fatal().Msgf("Failed to connect to systemd via DBus: %v", err)
	}

	h.dbusConn = conn

	return h
}

func (h *handler) initializeListener(ctx context.Context, topic notifier.Topic) {
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
		if !strings.HasSuffix(u.Name, ".service") && u.JobType != "service" {
			continue
		}
		// log.Info().Msgf("unit: %v job type: %v description: %v", u.Name, u.JobType, u.Description)

		m := toModel(u)
		if _, ok := h.status[m.Name]; ok {
			log.Warn().Msgf("conflicting name detected!?!")
			h.status[m.Name] = new(DBusUnitStatus)
		}

		func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.status[m.Name] = &m
		}()

		topic.Broadcast(ctx, Row[DBusUnitStatus]{
			Name: "unit",
			Key:  m.Name,
			Data: m,
		})
	}

	for _, v := range h.status {
		fmt.Printf("INID %-40s %-10s %-10s\n", v.Name, v.ActiveState, v.SubState)
	}

	fmt.Println("\n=== Watching for Service Changes ===")

	// Channel for DBus event notifications
	changes, errChan := conn.SubscribeUnits(0)

	for {
		select {
		case changedUnits := <-changes:
			for _, unit := range changedUnits {
				if unit == nil {
					log.Warn().Msgf("unit is nil, skipping %v", unit)
					continue
				}

				if !strings.HasSuffix(unit.Name, ".service") && unit.JobType != "service" {
					continue
				}
				// log.Info().Msgf("unit: %v job type: %v description: %v", u.Name, u.JobType, u.Description)

				fmt.Printf("[EVENT] %-40s  %-10s  %-10s\n",
					unit.Name,
					unit.ActiveState,
					unit.SubState,
				)

				// Save to memory
				m := toModel(*unit)

				func() {
					h.mu.Lock()
					defer h.mu.Unlock()
					h.status[m.Name] = &m
				}()

				topic.Broadcast(ctx, Row[DBusUnitStatus]{
					Name: "unit",
					Key:  m.Name,
					Data: m,
				})
			}
		case err := <-errChan:
			log.Err(err).Msgf("error received")
		case <-ctx.Done():
			return
		}
	}
}

// subscribe to systemd's service
func (h *handler) Subscribe() {

}

func toModel(status dbus.UnitStatus) DBusUnitStatus {
	return DBusUnitStatus{
		Name:        status.Name,
		Description: status.Description,
		LoadState:   status.LoadState,
		ActiveState: status.ActiveState,
		SubState:    status.SubState,
		Followed:    status.Followed,
		Path:        string(status.Path),
		JobId:       status.JobId,
		JobType:     status.JobType,
		JobPath:     string(status.JobPath),
	}
}
