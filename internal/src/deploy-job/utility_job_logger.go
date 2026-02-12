package deployjob

import (
	"context"
	"log/slog"

	"github.com/desain-gratis/common/lib/notifier"
)

const jobKey = "state"

type jobLogger struct {
	slog.Handler

	jobType string
	job     Job // check if it's feasible; if not, we use generic
	topic   notifier.Topic
}

// Logger with up-to-date state information
func NewJobLogger(topic notifier.Topic, jobType string, job Job) slog.Handler {
	return &jobLogger{
		Handler: slog.DiscardHandler,
		job:     job,
		jobType: jobType,
		topic:   topic,
	}
}

func (h *jobLogger) Handle(ctx context.Context, r slog.Record) error {
	h.topic.Broadcast(ctx, Log{Job: h.job, JobType: h.jobType, Record: r})
	return nil
}

func (h *jobLogger) Enabled(context.Context, slog.Level) bool { return true }

type Log struct {
	Job     Job
	JobType string
	Record  slog.Record
}
