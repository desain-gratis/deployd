package configurenginxunit

import (
	"context"
	"log/slog"
	"strings"

	"github.com/desain-gratis/common/lib/notifier"
	"github.com/desain-gratis/deployd/src/entity"
	"github.com/rs/zerolog/log"
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

func (w *localWorker) doSomethingAsLeader(ctx context.Context) {
	// w.log
	log.Info().Msgf("IM ZA LEADER, IM ZA LEADER; I DO LEADER CODE!!!!")
	<-ctx.Done()
	log.Info().Msgf("IM NOT ZA LEADER NOMORE!!!")
}

func (w *localWorker) initializeDeployment(out notifier.Topic, jobDefinition entity.DeploymentJob) {
	log := w.log

	var proceed bool
	for _, host := range jobDefinition.Target {
		proceed = proceed || host.Host == w.host.Host
	}
	if !proceed {
		log.Debug("received job that is not for this host")
		return
	}

	// local state
	state := &HostDeploymentJobState{
		ConfigurationStatus:  entity.HostConfigurationStatusPending,
		RestartServiceStatus: entity.HostRestartServiceStatusPending,
	}

	name := "configure-job-" + jobDefinition.Id
	log = w.log.
		With("namespace", jobDefinition.Ns).
		With("job_id", jobDefinition.Id).
		With("id", getKey(jobDefinition)). // instance id
		With("host", w.host.Host).
		With("name", name).
		With("state", state)

	// todo: prepare locking
	if _, ok := w.deploymentJobPool[getKey(jobDefinition)]; ok {
		return
	}

	if _, ok := jobDefinition.ConfigureHostJob.Status[w.host.Host]; !ok {
		// not part of the deployment worker
		log.Debug("received job that is not for this host")
		return
	}

	// Validate job state, if it's already configured, we wont execute

	ctx, cancel := context.WithCancel(context.Background())
	job := &configureJob{
		ctx:    ctx,
		cancel: cancel,

		topic:        out,
		host:         w.host,
		dependencies: w.dependencies,

		Job: jobDefinition,

		State: state,

		log: log,
	}

	w.deploymentJobPool[getKey(jobDefinition)] = job

	// TODO: use go-routine pooling / other library
	go job.startConfigureHost()
}

func getKey(job entity.DeploymentJob) string {
	keys := []string{job.Ns, job.Request.Service.Id, job.Id}
	return strings.Join(keys, "|")
}
