package configurewebsite

import (
	"context"
	"log/slog"
	"time"

	"github.com/desain-gratis/common/lib/notifier"
	configurewebsite "github.com/desain-gratis/deployd/internal/src/raft-app/configure-website"
	"github.com/desain-gratis/deployd/src/entity"
)

type localWorker struct {
	nginxUnitConfigAddress string

	dependencies *Dependencies
	host         *entity.Host

	// todo: can add lock
	deploymentJobPool map[string]*configureJob

	// todo: use input channel to queue job based on namespace & service
	// map[{ns, service}]chan <- msg

	// Also, retry mechanism / timeout handling, and resilliency should be coded
	// eg. after job is QUEUED, if local worker already setting up in memory & ready, local worker need to update to the raft back (we already implement)
	// what we havent is, if the local worker does not report back for various reason (only if they're expected to reply back)

	// controller level log
	log *slog.Logger

	// TODO: use worker pool B-)

	// TODO: later, after have many job types,
	// consider this localWorker can contain multiple types of job or just single

	// other types of job can be put here
}

// Local host state
type HostDeploymentJobState struct {
	ConfigurationStatus  entity.HostConfigurationStatus  `json:"configuration_status"`
	RestartServiceStatus entity.HostRestartServiceStatus `json:"restart_service_status"`
}

const minimumTimeOutIfConfiguredSeconds = 30

func (w *localWorker) configureWebsite(out notifier.Topic, jobDefinition *entity.JobConfigureWebsite) {
	log := w.log

	if jobDefinition == nil {
		log.Warn("empty job definition") // TODO: implement
		return
	}

	ctx := context.Background()
	updateTime := time.Now()
	_, err := w.dependencies.RaftConfigureWebsite.GenericUpdate(
		ctx,
		configurewebsite.Command_Host_ConfigureWebsiteResponse,
		entity.HostUpdateResult{
			Status:  entity.JobConfigureWebsiteStatusSuccess,
			Message: "success loh yaa",
			HostID:  w.host.Host,
			JobID:   jobDefinition.Id,
			Error:   nil,
			Time:    &updateTime,
		},
	)
	if err != nil {
		log.Warn("failed to notify configure success to manager. manager should check this host.", "error", err) // TODO: implement
		return
	}
}
