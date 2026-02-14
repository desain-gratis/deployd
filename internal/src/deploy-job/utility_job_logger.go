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
func NewNotifierLogger(topic notifier.Topic, base slog.Handler) slog.Handler {
	return &jobLogger{
		Handler: base, // TODO: discard handler for production ; can add toggle
		topic:   topic,
	}
}

func (h *jobLogger) Handle(ctx context.Context, r slog.Record) error {
	collect := map[string]any{
		"level": r.Level.String(),
		"time":  r.Time,
		"msg":   r.Message,
	}

	r.Attrs(func(a slog.Attr) bool {
		// TODO: more advanced value extraction later
		// if a.Key == "instance" {
		// 	switch value := a.Value.Any().(type) {
		// 	case *restartHostService:
		// 		collect[a.Key] = *value
		// 	case *configureHost:
		// 		collect[a.Key] = *value
		// 	}
		// } else {
		collect[a.Key] = a.Value.Any()
		// }
		return true
	})

	// use map for topic which is parsed here;
	// we parse here so that we can do early filtering
	h.topic.Broadcast(context.Background(), Log{Record: collect})

	return h.Handle(ctx, r)
}

func (h *jobLogger) Enabled(context.Context, slog.Level) bool { return true }

type Log struct {
	Record map[string]any
}
