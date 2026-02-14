package deployjob

import (
	"github.com/desain-gratis/common/lib/notifier"

	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
)

type eventHandler struct {
	jobsController *jobsController
	dependencies   *Dependencies
}

// StartConsumer exposed to main program
func (w *eventHandler) StartConsumer(topic notifier.Topic, subscription notifier.Subscription) {
	go func() {
		for event := range subscription.Listen() {
			switch value := event.(type) {
			case deployjob.EventDeploymentJobCreated:
				w.jobsController.configureHost(topic, value.Job)
			case deployjob.EventDeploymentJobCancelled:
				w.jobsController.cancelDeployment(topic, value.Job)
			case deployjob.EventRestartConfirmed:
				w.jobsController.restartService(topic, value)
			case deployjob.EventAllHostConfigured:
				w.jobsController.confirmDeploymentAsUserIfEnabled(topic, value)
			case deployjob.EventServiceRestarted:
				w.jobsController.continueRestartServiceAsUserIfEnabled(topic, value)

			default:
			}
		}
	}()
}
