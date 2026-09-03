package deployjob

import (
	"context"

	"github.com/desain-gratis/common/lib/notifier"

	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
)

type eventHandler struct {
	localWorker  *localWorker
	dependencies *Dependencies
}

// StartConsumer exposed to main program
func (w *eventHandler) StartConsumer(ctx context.Context, topic notifier.Topic, subscription notifier.Subscription) {
	// only start consuming event after raft is ready

	// wait ready
	// log.Info().Msgf("waiting job replica to be ready")

	// rCtx, ok := dgraft.GetRaftContext(ctx).(*runneretcd.RaftContext)
	// if !ok {
	// 	log.Fatal().Msgf("not an etcd raft runner")
	// }

	// Local worker
	go func() {
		// TODO: need to pass/wrap the raft's index, term, leader as well inside the event from producer (& the cluster ID)
		for event := range subscription.Listen() {
			switch value := event.(type) {
			case deployjob.EventDeploymentJobCreated:
				w.localWorker.initializeDeployment(topic, value.Job)
			case deployjob.EventDeploymentJobCancelled:
				w.localWorker.cancelDeployment(topic, value.Job)
			case deployjob.EventRestartConfirmed:
				w.localWorker.restartService(topic, value)
			case deployjob.EventAllHostConfigured:
				w.localWorker.confirmDeploymentAsUserIfEnabled(topic, value)
			case deployjob.EventServiceRestarted:
				w.localWorker.continueRestartServiceAsUserIfEnabled(topic, value)
			default:
			}
		}
	}()
}
