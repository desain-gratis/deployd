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
				w.jobsController.startConfigureJob(topic, value.Job) // test no goroutine
			case deployjob.EventDeploymentJobCancelled:
				w.jobsController.cancelConfigureJob(topic, value.Job)
			default:
			}
		}
	}()
}
