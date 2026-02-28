package deployjob

import (
	"context"

	"github.com/desain-gratis/common/lib/notifier"
	"github.com/desain-gratis/common/lib/raft"
	"github.com/desain-gratis/common/lib/raft/runner"
	"github.com/rs/zerolog/log"

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
	log.Info().Msgf("waiting job replica to be ready")

	initElu, raftSub, err := runner.WaitReady(ctx)
	if err != nil {
		log.Panic().Msgf("not ready")
	}
	log.Info().Msgf("replica ready replica id: %v shard id: %v term: %v leader id: %v",
		initElu.ReplicaID, initElu.ShardID, initElu.Term, initElu.LeaderID)

	// Leader events
	go func() {
		isCurrentTermLeader := initElu.LeaderID == initElu.ReplicaID
		currentTerm := initElu.Term

		var leaderCtx context.Context
		var cancelFn context.CancelFunc

		cancelFn = func() {} // init with noop

		if isCurrentTermLeader {
			leaderCtx, cancelFn = context.WithCancel(ctx)
			w.localWorker.doSomethingAsLeader(leaderCtx)
		}

		for msg := range raftSub.Listen() {
			elu, ok := msg.(raft.EventLeaderUpdate)
			if !ok {
				continue
			}

			if elu.Term <= currentTerm {
				// outdated!
				continue
			}
			currentTerm = elu.Term // update term
			isNextTermLeader := elu.LeaderID == initElu.ReplicaID

			// No leadership change
			if isCurrentTermLeader == isNextTermLeader {
				continue
			}

			// Stepped down
			if isCurrentTermLeader {
				isCurrentTermLeader = false
				cancelFn() // cancel current leader code
				continue
			}

			// Became leader
			isCurrentTermLeader = true
			leaderCtx, cancelFn = context.WithCancel(ctx)

			go func(ctx context.Context) {
				defer cancelFn()
				w.localWorker.doSomethingAsLeader(ctx)
			}(leaderCtx)
		}
	}()

	// Local worker
	go func() {
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
