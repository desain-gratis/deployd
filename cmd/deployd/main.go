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
	content_chraft "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/clickhouse-raft"
	"github.com/desain-gratis/deployd/internal/src/systemd"
	"github.com/desain-gratis/deployd/src/entity"
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
	enableArtifactdModule(ctx, router)
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

// enableArtifactDienableArtifactdModulescoveryModule enables upload artifact discovery / metadata query
func enableArtifactdModule(_ context.Context, router *httprouter.Router) {
	ctx, err := raft_runner.RunReplica[any](
		"artifactd-v1",
		content_chraft.New(nil,
			content_chraft.TableConfig{Name: "artifactd_repository", RefSize: 0},
			content_chraft.TableConfig{Name: "artifactd_build", RefSize: 1},
			content_chraft.TableConfig{Name: "artifactd_archive", RefSize: 2},
		),
	)
	if err != nil {
		log.Panic().Msgf("failed to run artifactd raft: %v", err)
	}

	baseURL := ""

	buildArtifactBlob, err := blob_s3.New(
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

	repositoryHandler := mycontentapi.NewFromStorage[*entity.Artifact](
		baseURL+"/artifactd/repository",
		nil,
		content_chraft.NewStorageClient(ctx, "artifactd_repository"),
		0,
	)

	buildHandler := mycontentapi.NewFromStorage[*entity.Artifact](
		baseURL+"/artifactd/build",
		[]string{"repository"},
		content_chraft.NewStorageClient(ctx, "artifactd_build"),
		1,
	)

	archiveHandler := mycontentapi.NewAttachment(
		mycontent_base.NewAttachment(
			content_chraft.NewStorageClient(ctx, "artifactd_archive"),
			2,
			buildArtifactBlob,
			false,
			"artifactd/archive",
		),
		baseURL+"/artifactd/archive",
		[]string{"repository", "build"},
		"",
	)

	router.POST("/artifactd/repository", repositoryHandler.Post)
	router.GET("/artifactd/repository", repositoryHandler.Get)
	router.DELETE("/artifactd/repository", repositoryHandler.Delete)

	router.POST("/artifactd/build", buildHandler.Post)
	router.GET("/artifactd/build", buildHandler.Get)
	router.DELETE("/artifactd/build", buildHandler.Delete)

	router.POST("/artifactd/archive", archiveHandler.Upload)
	router.GET("/artifactd/archive", archiveHandler.Get)
	router.DELETE("/artifactd/archive", archiveHandler.Delete)
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
