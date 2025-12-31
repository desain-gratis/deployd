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
	raft_replica "github.com/desain-gratis/common/lib/raft/replica"
	raft_runner "github.com/desain-gratis/common/lib/raft/runner"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	mycontentapi "github.com/desain-gratis/common/delivery/mycontent-api"
	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	blob_s3 "github.com/desain-gratis/common/delivery/mycontent-api/storage/blob/s3"
	content_clickhouse "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/clickhouse"
	"github.com/desain-gratis/deploy/internal/src/artifactd"
	"github.com/desain-gratis/deploy/internal/src/systemd"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Logger()
}

func main() {
	ctx, cancel := context.WithCancelCause(context.Background())

	initConfig()

	err := raft_replica.Init()
	if err != nil {
		log.Panic().Msgf("failed to init raft: %v", err)
	}

	wg := new(sync.WaitGroup)
	router := httprouter.New()

	enableSystemdModule(ctx, router)
	enableArtifactUploadModule(ctx, router)
	enableArtifactDiscoveryModule(ctx, router)
	enableUI(ctx, router)

	go startRouter(ctx, wg, router, config.GetString("http.public.address"))

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	log.Info().Msgf("WAITING FOR SIGINT :)")
	<-sigint
	cancel(errors.New("server closed"))
	wg.Wait()
	log.Info().Msgf("Bye bye")
}

func enableUI(ctx context.Context, router *httprouter.Router) {
	router.ServeFiles("/ui/*filepath", http.Dir(config.GetString("ui.dir")))
}

func enableSystemdModule(ctx context.Context, router *httprouter.Router) {
	topic := notifier_impl.NewStandardTopic()

	integration := systemd.New(ctx, topic)
	httpIntegration := systemd.Http(integration)

	router.GET("/ws", httpIntegration.StreamUnit)
}

// enableArtifactUploadModule enables a key-value store to store build artifact based on commit ID
func enableArtifactUploadModule(_ context.Context, router *httprouter.Router) {
	conn, err := config.getClickhouseConn("content")
	if err != nil {
		return
	}

	baseURL := config.GetString("http.public.fqdn")

	// storage for linux amd64 build artifact
	linuxAmd64Archive := content_clickhouse.New(conn, "linux_amd64", 0)
	linuxAmd64ArchiveBlob, err := blob_s3.New(
		config.GetString("storage.s3.blob.endpoint"),
		config.GetString("storage.s3.blob.key_id"),
		config.GetString("storage.s3.blob.key_secret"),
		config.GetBool("storage.s3.blob.use_ssl"),
		config.GetString("storage.s3.blob.bucket_name"),
		config.GetString("storage.s3.blob.base_public_url"),
	)
	if err != nil {
		log.Fatal().Msgf("failure to create blob storage client: %v", err)
	}

	linuxAmd64Handler := mycontentapi.NewAttachment(
		mycontent_base.NewAttachment(
			linuxAmd64Archive,
			2,
			linuxAmd64ArchiveBlob,
			false,               // hide the s3 URL
			"assets/user/image", // the location in the s3 compatible bucket
		),
		baseURL+"/org/user/thumbnail",
		[]string{},
		"",
	)

	// <os>/<arch>
	router.GET("/artifactd/linux/amd64", linuxAmd64Handler.Get)
	router.POST("/artifactd/linux/amd64", linuxAmd64Handler.Upload)
	router.DELETE("/artifactd/linux/amd64", linuxAmd64Handler.Delete)
}

// enableArtifactDiscoveryModule enables upload artifact discovery / metadata query
func enableArtifactDiscoveryModule(_ context.Context, router *httprouter.Router) {
	raft_runner.ForEachReplica[any]("artifactd", func(ctx context.Context) error {
		outbox := notifier_impl.NewStandardTopic()

		raftHttpHandler := artifactd.NewHttpRaftHandler(ctx, outbox)

		router.POST("/artifactd/register", raftHttpHandler.RegisterArtifact)
		router.GET("/artifactd/discover", raftHttpHandler.DiscoverArtifact)
		router.GET("/artifactd/ws", raftHttpHandler.StreamAll)

		sapp := artifactd.NewRaft()

		return raft_runner.Run(ctx, sapp)
	})
}

// TODO: refactor for multiple http server
func startRouter(ctx context.Context, wg *sync.WaitGroup, router *httprouter.Router, address string) {
	// global cors handlign
	router.HandleOPTIONS = true
	router.GlobalOPTIONS = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	withCors := func(router http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()
			header.Set("Access-Control-Allow-Methods", "*")
			header.Set("Access-Control-Allow-Origin", "*")
			header.Set("Access-Control-Allow-Methods", "*")
			header.Set("Access-Control-Allow-Headers", "*")
			router.ServeHTTP(w, r)
		})
	}

	wsWg := &sync.WaitGroup{}
	server := &http.Server{
		Addr:    address,
		Handler: withCors(router),
		// ReadTimeout: 2 * time.Second,

		// important: do not set WriteTimeout if we enable long running connection like this example
		// WriteTimeout: 15 * time.Second,

		BaseContext: func(l net.Listener) context.Context {
			ctx := context.WithValue(ctx, "ws-wg", wsWg)
			return ctx
		},
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()

		// close HTTP connection
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		log.Info().Msgf("Shutting down HTTP server..")
		if err := server.Shutdown(ctx); err != nil {
			// Error from closing listeners, or context timeout:
			log.Err(err).Msgf("HTTP server Shutdown")
		}

		log.Info().Msgf("Waiting for websocket connection to close..")
		wsWg.Wait()
	}()

	// TODO: maybe can use this for more graceful handling
	// server.RegisterOnShutdown()

	log.Info().Msgf("Serving at %v..\n", server.Addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatal().Msgf("HTTP server ListendAndServe: %v", err)
	}
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
