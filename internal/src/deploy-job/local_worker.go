package deployjob

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/desain-gratis/common/lib/notifier"
	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
	"github.com/desain-gratis/deployd/src/entity"
	"github.com/rs/zerolog/log"
)

type localWorker struct {
	dependencies *Dependencies
	host         *entity.Host

	// todo: can add lock
	deploymentJobPool map[string]*deploymentJob

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
	job := &deploymentJob{
		ctx:    ctx,
		cancel: cancel,

		topic:        out,
		host:         w.host,
		dependencies: w.dependencies,

		Job: jobDefinition,

		State: state,

		jobBase: &jobBase{ // ignore this first
			Status:      StatusPending,
			Name:        name,
			RetryCount:  0,
			CurrentStep: 0,
		},

		log: log,
	}

	w.deploymentJobPool[getKey(jobDefinition)] = job

	// TODO: use go-routine pooling / other library
	go job.startConfigureHost()
}

func (w *localWorker) cancelDeployment(_ notifier.Topic, jobDefinition entity.DeploymentJob) {
	job, ok := w.deploymentJobPool[getKey(jobDefinition)]
	if !ok {
		return
	}

	// TODO: implement

	job.cancel()
}

func (w *localWorker) confirmDeploymentAsUserIfEnabled(_ notifier.Topic, event deployjob.EventAllHostConfigured) {
	log := w.log

	if !event.Job.Request.IsBelieve {
		// let user do the confirmation
		return
	}

	// we believe, let's gooo

	// only one node is enough for executing this
	if event.TriggerHost != w.host.Host {
		log.Info("confirming job deployment on behalf of user (believe)", "host", w.host.Host)
		return
	}

	log.Info("confirming job deployment on behalf of user (believe)", "host", w.host.Host)
	result, err := w.dependencies.RaftJobUsecase.ConfirmRestartService(context.Background(), deployjob.RestartConfirmation{
		Ns:        event.Job.Ns,
		JobId:     event.Job.Id,
		Service:   event.Job.Request.Service.Id,
		Message:   "LGTM",
		Agent:     "saya-bot-believe:" + w.host.Host,
		CreatedAt: time.Now(),
	})
	if err != nil {
		log.Error("failed to confirm deployment job status automatically", "error", err)
		return
	}

	log.Info("successfully confirmed job deployment on behalf of user",
		"host", w.host.Host, "job_status", result.Job.Status, "current_step", result.Step)
}

func (w *localWorker) continueRestartServiceAsUserIfEnabled(_ notifier.Topic, event deployjob.EventServiceRestarted) {
	log := w.log

	if event.Job.Status == entity.DeploymentJobStatusDeployed {
		// log.Info("all service has been restarted successfully")
		return
	}

	if !event.Job.Request.IsBelieve {
		// let user do the confirmation
		log.Info("received request to restart service as user. ignoring because the job need manual user confirmation")
		return
	}

	// we believe, let's gooo

	// only one node is enough for executing this
	if event.TriggerHost != w.host.Host {
		log.Info("received request to restart service as user. ignoring because of we're not the assigned executor.")
		return
	}

	log.Info("continuing job deployment on behalf of user (believe)")
	result, err := w.dependencies.RaftJobUsecase.ConfirmRestartService(context.Background(), deployjob.RestartConfirmation{
		Ns:        event.Job.Ns,
		JobId:     event.Job.Id,
		Service:   event.Job.Request.Service.Id,
		Message:   "LGTM",
		Agent:     "saya-bot-believe:" + w.host.Host,
		CreatedAt: time.Now(),
	})
	if err != nil {
		log.Error("failed to confirm deployment job status automatically", "error", err)
		return
	}

	log.Info("successfully continuing job deployment on behalf of user",
		"host", w.host.Host, "job_status", result.Job.Status, "current_step", result.Step)
}

func (w *localWorker) restartService(_ notifier.Topic, event deployjob.EventRestartConfirmed) {
	log := w.log
	job, ok := w.deploymentJobPool[getKey(event.Job)]
	if !ok {
		// should not be possibre, if we already configure, the job should still be there
		log.Warn("job pool should be there for restart")
		return
	}

	// restart is one by one and targeted
	if event.TargetHost != w.host.Host {
		log.Info("received request to restart service, but it's not our turn yet, so it is ignored")
		return
	}

	job.startRestartHostService()
}

func getKey(job entity.DeploymentJob) string {
	keys := []string{job.Ns, job.Request.Service.Id, job.Id}
	return strings.Join(keys, "|")
}
