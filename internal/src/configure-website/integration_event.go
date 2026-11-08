package configurewebsite

import (
	"context"
	"log"

	"github.com/desain-gratis/common/lib/notifier"

	configurewebsite "github.com/desain-gratis/deployd/internal/src/raft-app/configure-website"
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
				case configurewebsite.EventJobSubmitted:
					err := w.localWorker.configureWebsite(topic, value.Job)
					if err != nil {
						log.Printf("error configure website (%v): %v", value.Job.Id, err)
						continue
					}
					log.Printf("successfully configured website %v", value.Job.Id)
				default:
				}
			case _ = <-ctx.Done():
				return
			}

		}
	}()
}
