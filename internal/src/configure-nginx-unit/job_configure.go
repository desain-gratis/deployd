package configurenginxunit

import (
	"context"
	"log/slog"

	"github.com/desain-gratis/common/lib/notifier"
	"github.com/desain-gratis/deployd/src/entity"
)

// shared state for integration
// represents an in-memory job / process inside a host.
type configureJob struct {
	// Global context
	ctx    context.Context
	cancel context.CancelFunc

	dependencies *Dependencies
	topic        notifier.Topic
	log          *slog.Logger
	host         *entity.Host

	State *HostDeploymentJobState `json:"state"` // mutable state

	Job entity.DeploymentJob `json:"job"`
}

func (d *configureJob) startConfigureHost() {
	log := d.log

	ctx, cancel := context.WithCancel(d.ctx)
	defer cancel()

	log.Info("received request to start configure host")

	_, err := d.dependencies.RaftNginxUnitUsecase.HostNotifyUpdateNginxUnitConfigResult(ctx, "success loh yaa")
	if err != nil {
		log.Warn("failed to notify configure success to manager. manager should check this host.", "error", err) // TODO: implement
		return
	}
}
