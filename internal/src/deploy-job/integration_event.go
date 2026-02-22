package deployjob

import (
	"github.com/desain-gratis/common/lib/notifier"

	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
)

type eventHandler struct {
	localWorker  *localWorker
	dependencies *Dependencies
}

// StartConsumer exposed to main program
func (w *eventHandler) StartConsumer(topic notifier.Topic, subscription notifier.Subscription) {
	go func() {
		for event := range subscription.Listen() {
			switch value := event.(type) {
			case deployjob.EventDeploymentJobCreated:
				w.localWorker.initializeDeployment(topic, value.Job)
				// TODO:
				// localManager checkDeployTimeout (eg. check at certaina point, configure finished; and the whole finished)
				// manager will spawn timeAfter.
				//
				// time.AfterFunc(time.Duration(*d.Job.Request.TimeoutSeconds)*time.Second, func(){
				//	 check if replica leader for "job" entity,
				//   and do final check to node;
				// })
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
