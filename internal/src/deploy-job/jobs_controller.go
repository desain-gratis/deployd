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

type pair struct {
	*configureJob
	cancel context.CancelFunc
}
type jobsController struct {
	dependencies *Dependencies
	host         *entity.Host

	configureJobPool map[string]*pair

	// TODO: use worker pool B-)

	// TODO: later, after have many job types,
	// consider this jobsController can contain multiple types of job or just single

	// other types of job can be put here
}

const minimumTimeOutIfConfiguredSeconds = 30

func (w *jobsController) startConfigureJob(out notifier.Topic, jobDefinition entity.DeploymentJob) {
	// todo: prepare locking
	if _, ok := w.configureJobPool[getKey(jobDefinition)]; ok {
		return
	}

	// Validate job state, if it's already configured, we wont execute

	ctx, cancel := context.WithCancel(context.Background())

	if jobDefinition.Request.TimeoutSeconds != nil && *jobDefinition.Request.TimeoutSeconds >= minimumTimeOutIfConfiguredSeconds {
		time.AfterFunc(time.Duration(*jobDefinition.Request.TimeoutSeconds)*time.Second, cancel)
	}

	job := &configureJob{
		ctx:          ctx,
		cancel:       cancel,
		topic:        out,
		host:         w.host,
		dependencies: w.dependencies,

		Status:      StatusPending,
		Job:         jobDefinition,
		Name:        "configure-job-" + jobDefinition.Id,
		RetryCount:  0,
		CurrentStep: 0,
	}

	// logger that forwards to topic
	logger := slog.New(NewJobLogger(out, "configure", job))

	job.log = logger

	// insert into job pool
	w.configureJobPool[getKey(jobDefinition)] = &pair{configureJob: job, cancel: cancel}

	// Report to job manager (raft) that this host are starting the Configuring & Installing job
	err := w.dependencies.RaftJobUsecase.FeedHostConfigurationUpdate(ctx, deployjob.ConfigurationUpdateRequest{
		Ns:        jobDefinition.Ns,
		Id:        jobDefinition.Id,
		Service:   jobDefinition.Request.Service.Id,
		HostName:  w.host.Host,
		Status:    entity.DeploymentJobStatusConfiguring,
		Message:   "Configurating and installing",
		URL:       "", // specific
		CreatedAt: time.Now(),
	})
	if err != nil {
		job.Status = StatusFailed
		log.Err(err).Msgf("failed to update job configuration status %v", err)
		return
	}

	go func() {
		jobResult := entity.DeploymentJobStatusFailed

		job.Execute(ctx)

		switch job.GetStatus() {
		case StatusSuccess:
			jobResult = entity.DeploymentJobStatusConfigured
		case StatusFailed:
			jobResult = entity.DeploymentJobStatusFailed
		case StatusCancelled:
			jobResult = entity.DeploymentJobStatusCancelled
		default:
			jobResult = entity.DeploymentJobStatusInvalid
		}

		// Report back to job manager (raft)
		err = w.dependencies.RaftJobUsecase.FeedDeploymentUpdate(ctx, deployjob.DeploymentUpdateRequest{
			Ns:        jobDefinition.Ns,
			Id:        jobDefinition.Id,
			Service:   jobDefinition.Request.Service.Id,
			HostName:  w.host.Host,
			Status:    jobResult,
			UpdatedAt: time.Now(),
		})
		if err != nil {
			log.Err(err).Msgf("failed to update deployment job status %v", err)
			return
		}
	}()
}

func (w *jobsController) cancelConfigureJob(_ notifier.Topic, jobDefinition entity.DeploymentJob) {
	// todo: prepare locking
	pair, ok := w.configureJobPool[getKey(jobDefinition)]
	if !ok {
		return
	}

	pair.cancel()
}

func getKey(job entity.DeploymentJob) string {
	keys := []string{job.Ns, job.Request.Service.Id, job.Id}
	return strings.Join(keys, "\\")
}
