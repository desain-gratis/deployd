package deployjob

import (
	"context"
	"log/slog"
	"maps"

	"github.com/desain-gratis/common/lib/notifier"
)

const jobKey = "state"

type jobLogger struct {
	slog.Handler

	topic notifier.Topic
	attrs map[string]any
}

// Logger with up-to-date state information
// todo: optimize / or use another library; to store the log in memory (not render immedaitely)
func NewNotifierLogger(topic notifier.Topic) slog.Handler {
	return &jobLogger{
		Handler: slog.DiscardHandler,
		topic:   topic,
		attrs:   make(map[string]any),
	}
}

func (h *jobLogger) Handle(ctx context.Context, r slog.Record) error {
	collect := map[string]any{
		"level": r.Level.String(),
		"time":  r.Time,
		"msg":   r.Message,
	}

	maps.Copy(collect, h.attrs)

	r.Attrs(func(a slog.Attr) bool {
		collect[a.Key] = a.Value.Any()
		return true
	})

	// use map for topic which is parsed here;
	// we parse here so that we can do early filtering
	h.topic.Broadcast(ctx, Log(collect))

	return nil
}

func (h *jobLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	// todo: optimize / or use another library; to store the log in memory (not render immedaitely)
	for _, attr := range attrs {
		h.attrs[attr.Key] = attr.Value.Any()
	}
	return h
}

func (h *jobLogger) WithGroup(name string) slog.Handler {
	// not supported
	return h
}

func (h *jobLogger) Enabled(context.Context, slog.Level) bool { return true }

type Log map[string]any
