package configurenginxunit

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
	// Local worker
	go func() {
		// TODO: need to pass/wrap the raft's index, term, leader as well inside the event from producer (& the cluster ID)
		for {
			select {
			case event, _ := <-subscription.Listen():
				switch value := event.(type) {
				case deployjob.EventDeploymentJobCreated:
					w.localWorker.initializeDeployment(topic, value.Job)
				default:
				}
			case _ = <-ctx.Done():
				return
			}

		}
	}()
}
