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

	// sub-job that we manage
	configureHost      *configureHost
	restartHostService *restartHostService

	Job entity.DeploymentJob `json:"job"`
}

func (d *deploymentJob) startConfigureHost() {
	// log := d.log
	ctx, cancel := context.WithCancel(d.ctx)
	defer cancel()

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
		// log.Warn("failed to notify configure state to manager.", "error", err)
	}

	if d.Job.Request.TimeoutSeconds != nil && *d.Job.Request.TimeoutSeconds >= minimumTimeOutIfConfiguredSeconds {
		time.AfterFunc(time.Duration(*d.Job.Request.TimeoutSeconds)*time.Second, cancel)
	}

	d.configureHost = &configureHost{
		deploymentJob: d,
		ctx:           ctx,
		cancel:        cancel,
		status:        entity.HostConfigurationStatusPending,
	}

	d.configureHost.log = d.log.With("node", "configure-host").
		With("status", d.configureHost.status).
		With("address", getKey(d.Job)).
		With("instance", d.configureHost)

	// log.Info("configuring host")

	var errMsg *string
	err = d.configureHost.Execute()
	if err != nil {
		d.configureHost.status = entity.HostConfigurationStatusFailed
		if errors.Is(err, context.Canceled) {
			d.configureHost.status = entity.HostConfigurationStatusCancelled
		}
		errStr := err.Error()
		errMsg = &errStr
	} else {
		d.configureHost.status = entity.HostConfigurationStatusSuccess
	}

	// Report back to job manager (raft)
	_, err = d.dependencies.RaftJobUsecase.FeedHostConfigurationUpdate(d.ctx, deployjob.ConfigurationUpdateRequest{
		Ns:           d.Job.Ns,
		JobId:        d.Job.Id,
		Service:      d.Job.Request.Service.Id,
		HostName:     d.host.Host,
		Status:       d.configureHost.status,
		ErrorMessage: errMsg,
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		// again, no need to report back to Raft; if timeout, they should check the node whether it's successful or failed.
		// if not either success or failed (eg. in progress), we say it's invalid / undefined.
		// if success, raft need help to confirm (because this path is returning error)
		// log.Warn("failed to notify configure success to manager. manager should check this host.", "error", err) // TODO: implement
		return
	}
	// log.Info("successfully configuring host")
}

func (d *deploymentJob) startRestartHostService() {
	log := d.log

	log.Info("received request to restart service")

	ctx, cancel := context.WithCancel(d.ctx)
	defer cancel()

	d.restartHostService = &restartHostService{
		deploymentJob: d,
		ctx:           ctx,
		cancel:        cancel,
		status:        entity.HostDeploymentStatusRestarting,
	}

	d.restartHostService.log = d.log.With("node", "restart-service").
		With("status", d.restartHostService.status).
		With("instance", d.restartHostService)

	// Report back to job manager (raft)
	_, err := d.dependencies.RaftJobUsecase.FeedHostRestartServiceUpdate(d.ctx, deployjob.HostRestartServiceUpdateRequest{
		Ns:        d.Job.Ns,
		JobId:     d.Job.Id,
		Service:   d.Job.Request.Service.Id,
		HostName:  d.host.Host,
		Status:    d.restartHostService.status,
		UpdatedAt: time.Now(),
	})
	if err != nil {
		log.Warn("failed to notify deployment state to manager.", "error", err)
	}

	log.Info("restarting service")
	var errMsg *string

	err = d.restartHostService.Execute()
	if err != nil {
		d.restartHostService.status = entity.HostDeploymentStatusFailed
		if errors.Is(err, context.Canceled) {
			d.restartHostService.status = entity.HostDeploymentStatusTimeOut
		}
		errStr := err.Error()
		errMsg = &errStr
	} else {
		d.restartHostService.status = entity.HostDeploymentStatusSuccess
	}
	// Report back to job manager (raft)
	_, err = d.dependencies.RaftJobUsecase.FeedHostRestartServiceUpdate(d.ctx, deployjob.HostRestartServiceUpdateRequest{
		Ns:           d.Job.Ns,
		JobId:        d.Job.Id,
		Service:      d.Job.Request.Service.Id,
		HostName:     d.host.Host,
		Status:       d.restartHostService.status,
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

	log.Info("successfully restarting service")
}
