package deployjob

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/desain-gratis/common/lib/notifier"
	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
	"github.com/desain-gratis/deployd/src/entity"
)

type jobsController struct {
	dependencies *Dependencies
	host         *entity.Host

	// todo: can add lock
	deploymentJobPool map[string]*deploymentJob

	// controller level log
	log *slog.Logger

	// TODO: use worker pool B-)

	// TODO: later, after have many job types,
	// consider this jobsController can contain multiple types of job or just single

	// other types of job can be put here
}

const minimumTimeOutIfConfiguredSeconds = 30

func (w *jobsController) configureHost(out notifier.Topic, jobDefinition entity.DeploymentJob) {
	// todo: prepare locking
	if _, ok := w.deploymentJobPool[getKey(jobDefinition)]; ok {
		return
	}

	if _, ok := jobDefinition.Configuration.Status[w.host.Host]; !ok {
		// not part of the deployment worker
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

		jobBase: &jobBase{
			Status:      StatusPending,
			Name:        "configure-job-" + jobDefinition.Id,
			RetryCount:  0,
			CurrentStep: 0,
		},
	}

	// logger that forwards to topic
	baseLogger := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})

	logger := slog.New(baseLogger).
		With("namespace", jobDefinition.Ns).
		With("job_id", jobDefinition.Id).
		With("status", jobDefinition.Status).
		With("node", "controller").
		With("instance", job)

	job.log = logger

	// insert into job pool
	w.deploymentJobPool[getKey(jobDefinition)] = job

	// TODO: use go-routine pooling / other library
	go job.startConfigureHost()
}

func (w *jobsController) cancelDeployment(_ notifier.Topic, jobDefinition entity.DeploymentJob) {
	job, ok := w.deploymentJobPool[getKey(jobDefinition)]
	if !ok {
		return
	}

	job.cancel()
}

func (w *jobsController) confirmDeploymentAsUserIfEnabled(_ notifier.Topic, event deployjob.EventAllHostConfigured) {
	if !event.Job.Request.IsBelieve {
		// let user do the confirmation
		return
	}

	// we believe, let's gooo

	// only one node is enough for executing this
	if event.TriggerHost != w.host.Host {
		return
	}

	log := w.log

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

func (w *jobsController) continueRestartServiceAsUserIfEnabled(_ notifier.Topic, event deployjob.EventServiceRestarted) {
	log := w.log

	if event.Job.Status == entity.DeploymentJobStatusDeployed {
		// log.Info("all service has been restarted successfully")
		return
	}

	if !event.Job.Request.IsBelieve {
		// let user do the confirmation
		return
	}

	// we believe, let's gooo

	// only one node is enough for executing this
	if event.TriggerHost != w.host.Host {
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

func (w *jobsController) restartService(_ notifier.Topic, event deployjob.EventRestartConfirmed) {
	job, ok := w.deploymentJobPool[getKey(event.Job)]
	if !ok {
		// should not be possibre, if we already configure, the job should still be there
		return
	}

	if event.TargetHost == w.host.Host {
		job.startRestartHostService()
	}
}

func getKey(job entity.DeploymentJob) string {
	keys := []string{job.Ns, job.Request.Service.Id, job.Id}
	return strings.Join(keys, "\\")
}
