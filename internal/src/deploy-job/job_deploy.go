package deployjob

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/desain-gratis/common/lib/notifier"
	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
	"github.com/desain-gratis/deployd/src/entity"
)

var _ Job = &deploymentJob{}

// shared state for integration
// represents an in-memory job / process inside a host.
type deploymentJob struct {
	*jobBase

	// Global context
	ctx    context.Context
	cancel context.CancelFunc

	dependencies *Dependencies
	topic        notifier.Topic
	log          *slog.Logger
	host         *entity.Host

	State *HostDeploymentJobState `json:"state"` // mutable state

	// sub-job that we manage
	configureHost      *configureHost
	restartHostService *restartHostService

	Job entity.DeploymentJob `json:"job"`
}

func (d *deploymentJob) startConfigureHost() {
	log := d.log

	ctx, cancel := context.WithCancel(d.ctx)
	defer cancel()

	log.Info("received request to start configure host")

	// modify local state
	d.State.ConfigurationStatus = entity.HostConfigurationStatusConfiguring

	// Report to job manager (raft) that this host are starting the Configuring & Installing job
	_, err := d.dependencies.RaftJobUsecase.FeedHostConfigurationUpdate(d.ctx, deployjob.ConfigurationUpdateRequest{
		Ns:        d.Job.Ns,
		JobId:     d.Job.Id,
		Service:   d.Job.Request.Service.Id,
		HostName:  d.host.Host,
		Status:    entity.HostConfigurationStatusConfiguring,
		URL:       "", // specific
		UpdatedAt: time.Now(),
	})
	if err != nil {
		// when time out, the checking thread the timeout is failed or not later; so
		// no need to report immediately to Raft during failure;
		// or we can later
		log.Warn("failed to notify configuring state to manager.", "error", err)
	}

	if d.Job.Request.TimeoutSeconds != nil && *d.Job.Request.TimeoutSeconds >= minimumTimeOutIfConfiguredSeconds {
		time.AfterFunc(time.Duration(*d.Job.Request.TimeoutSeconds)*time.Second, cancel)
	}

	d.configureHost = &configureHost{
		deploymentJob: d,
		ctx:           ctx,
		cancel:        cancel,
		log:           log,
	}

	log.Info("configuring host")

	terminalStatus := entity.HostConfigurationStatusSuccess

	var errMsg *string
	err = d.configureHost.Execute()
	if err != nil {
		terminalStatus = entity.HostConfigurationStatusFailed
		if errors.Is(err, context.Canceled) {
			terminalStatus = entity.HostConfigurationStatusCancelled
		}
		errStr := err.Error()
		errMsg = &errStr
		log.Error("failed to configure host: " + errStr)
	} else {
		log.Info("successfully configure host")
	}

	d.State.ConfigurationStatus = terminalStatus

	// Report back to job manager (raft);
	// only the terminal state, if raft want much detailed state, they need to hit the local worker endpoint TODO
	_, err = d.dependencies.RaftJobUsecase.FeedHostConfigurationUpdate(d.ctx, deployjob.ConfigurationUpdateRequest{
		Ns:           d.Job.Ns,
		JobId:        d.Job.Id,
		Service:      d.Job.Request.Service.Id,
		HostName:     d.host.Host,
		Status:       terminalStatus,
		ErrorMessage: errMsg,
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		// again, no need to report back to Raft; if timeout, they should check the node whether it's successful or failed.
		// if not either success or failed (eg. in progress), we say it's invalid / undefined.
		// if success, raft need help to confirm (because this path is returning error)
		log.Warn("failed to notify configure success to manager. manager should check this host.", "error", err) // TODO: implement
		return
	}
}

func (d *deploymentJob) startRestartHostService() {
	log := d.log

	ctx, cancel := context.WithCancel(d.ctx)
	defer cancel()

	d.restartHostService = &restartHostService{
		deploymentJob: d,
		ctx:           ctx,
		cancel:        cancel,
		log:           log,
	}

	log.Info("received request to restart service")

	// modify local state
	d.State.RestartServiceStatus = entity.HostRestartServiceStatusRestarting

	// Report back to job manager (raft)
	_, err := d.dependencies.RaftJobUsecase.FeedHostRestartServiceUpdate(d.ctx, deployjob.HostRestartServiceUpdateRequest{
		Ns:        d.Job.Ns,
		JobId:     d.Job.Id,
		Service:   d.Job.Request.Service.Id,
		HostName:  d.host.Host,
		Status:    entity.HostRestartServiceStatusRestarting,
		UpdatedAt: time.Now(),
	})
	if err != nil {
		log.Warn("failed to notify deployment state to manager.", "error", err)
	}

	log.Info("restarting service")
	var errMsg *string

	terminalStatus := entity.HostRestartServiceStatusSuccess
	err = d.restartHostService.Execute()
	if err != nil {
		terminalStatus = entity.HostRestartServiceStatusFailed
		if errors.Is(err, context.Canceled) {
			terminalStatus = entity.HostRestartServiceStatusTimeOut
		}
		errStr := err.Error()
		errMsg = &errStr
		log.Error("failed to restart service: " + errStr)
	} else {
		log.Info("successfully restarting service")
	}

	d.State.RestartServiceStatus = terminalStatus

	// Report back to job manager (raft)
	_, err = d.dependencies.RaftJobUsecase.FeedHostRestartServiceUpdate(d.ctx, deployjob.HostRestartServiceUpdateRequest{
		Ns:           d.Job.Ns,
		JobId:        d.Job.Id,
		Service:      d.Job.Request.Service.Id,
		HostName:     d.host.Host,
		Status:       terminalStatus,
		ErrorMessage: errMsg,
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		// again, no need to report back to Raft; if timeout, they should check the node whether it's successful or failed.
		// if not either success or failed (eg. in progress), we say it's invalid / undefined.
		// if success, raft need help to confirm (because this path is returning error)
		log.Warn("failed to notify deployment status to manager. manager should check this host.", "error", err) // TODO: implement
		return
	}
}
