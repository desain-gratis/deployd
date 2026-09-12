package configurewebsite

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
