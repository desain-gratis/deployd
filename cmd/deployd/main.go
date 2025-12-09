package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	notifier_impl "github.com/desain-gratis/common/lib/notifier/impl"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/desain-gratis/deploy/internal/src/systemd"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Logger()
}

func main() {
	ctx, cancel := context.WithCancelCause(context.Background())

	wg := new(sync.WaitGroup)

	enableSystemdModule(ctx, wg)

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	log.Info().Msgf("WAITING FOR SIGINT :)")
	<-sigint
	cancel(errors.New("server closed"))
	wg.Wait()
	log.Info().Msgf("Bye bye")
}

func enableSystemdModule(ctx context.Context, wg *sync.WaitGroup) {
	router := httprouter.New()
	topic := notifier_impl.NewStandardTopic()

	integration := systemd.New(ctx, topic)
	httpIntegration := systemd.Http(integration)

	router.GET("/ws", httpIntegration.StreamUnit)

	wsWg := &sync.WaitGroup{}
	server := http.Server{
		Addr:        ":9999",
		Handler:     withCors(router),
		ReadTimeout: 2 * time.Second,

		// important: do not set WriteTimeout if we enable long running connection like this example
		// WriteTimeout: 15 * time.Second,

		BaseContext: func(l net.Listener) context.Context {
			// inject with application context.
			ctx := context.WithValue(ctx, "ws-wg", wsWg)
			return ctx
		},
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			// Error starting or closing listener:
			log.Fatal().Msgf("HTTP server ListendAndServe: %v", err)
		}
	}()

	go func() {
		// wait main context to finish
		<-ctx.Done()

		// wait for http server shutdown
		closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(closeCtx); err != nil {
			// Error from closing listeners, or context timeout:
			log.Err(err).Msgf("HTTP server Shutdown")
		}

		// now, wait for closed websocket connection
		// web socket uses ctx, so it should be the same
		wsWg.Wait()
	}()
}

func withCors(router http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Access-Control-Allow-Methods", header.Get("Allow"))
		header.Set("Access-Control-Allow-Origin", "*")
		// header.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		header.Set("Access-Control-Allow-Headers", "Content-Type")
		router.ServeHTTP(w, r)
	})
}
