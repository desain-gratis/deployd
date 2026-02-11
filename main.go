package main

import (
	"context"
	"log/slog"
	"os"
	"time"
)

type job struct {
	Progress float32 `json:"progress"`

	internal string
}

type stateLogger struct {
	slog.Handler
	state *job
}

// Logger with up-to-date state information
func NewStateLogger(base slog.Handler, state *job) slog.Handler {
	return &stateLogger{
		Handler: base,
		state:   state,
	}
}

func (h *stateLogger) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(slog.Any(stateKey, *h.state))
	return h.Handler.Handle(ctx, r)
}

const stateKey string = "state"

func main() {
	job := &job{Progress: 0.1, internal: "secret"}
	stateLogger := NewStateLogger(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}), job)

	logger := slog.New(stateLogger)

	logger.Info("starting..")
	time.Sleep(500 * time.Millisecond)
	job.Progress = 0.5
	logger.Info("halfway")

	time.Sleep(500 * time.Millisecond)
	job.Progress = 0.6
	logger.Info("..")

	time.Sleep(500 * time.Millisecond)
	job.Progress = 0.7
	logger.Info("...")

	time.Sleep(500 * time.Millisecond)
	job.Progress = 0.9
	logger.Info("...")

	time.Sleep(500 * time.Millisecond)
	job.Progress = 0.95
	logger.Info("almost!")
	time.Sleep(2000 * time.Millisecond)

	job.Progress = 1
	logger.Info("done :)")

}
