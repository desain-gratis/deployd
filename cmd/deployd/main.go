package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	mycontentapi "github.com/desain-gratis/common/delivery/mycontent-api"
	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	blob_s3 "github.com/desain-gratis/common/delivery/mycontent-api/storage/blob/s3"
	content_chraft "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/clickhouse-raft"
	notifier_impl "github.com/desain-gratis/common/lib/notifier/impl"
	raft_runner "github.com/desain-gratis/common/lib/raft/runner"

	"github.com/desain-gratis/deployd/internal/src/systemd"
	"github.com/desain-gratis/deployd/src/deployd"
	"github.com/desain-gratis/deployd/src/entity"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Logger()
}

var (
	publicBaseURL string

	// host specific configuration
	hostConfigUsecase *mycontent_base.Handler[*entity.Host]

	// service configuration (to be installed on the host)
	serviceDefinitionUsecase *mycontent_base.Handler[*entity.ServiceDefinition]

	// archive / artifact repository storing build & archive information
	repositoryUsecase *mycontent_base.Handler[*entity.Repository]

	// store service's env
	envUsecase *mycontent_base.Handler[*entity.Env]

	// store service's secret
	secretUsecase *mycontent_base.Handler[*entity.Secret]
)

func main() {
	ctx, cancel := context.WithCancelCause(context.Background())

	initConfig()

	err := deployd.InjectSecretToViper(config.Viper)
	if err != nil && !errors.Is(err, deployd.ErrNotConfigured) {
		log.Panic().Msgf("failed to merge config with secret: %v", err)
	} else if err != nil {
		log.Warn().Msgf("not in deployd environment")
	}

	err = deployd.InitializeRaft(nil)
	if err != nil {
		log.Panic().Msgf("failed to init raft: %v", err)
	}

	wg := new(sync.WaitGroup)
	router := httprouter.New()

	// enableSystemdModule(ctx, router)
	enableArtifactdModule(ctx, router)
	enableDeploydModule(ctx, router)
	enableSecretdModule(ctx, router)
	enableUI(ctx, router)

	err = initWithTestData(ctx)
	if err != nil {
		log.Err(err).Msgf("failed to initialize starting data in deployd")
	}

	// update host config first

	go startRouter(ctx, wg, router, config.GetString("http.public.address"))

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	log.Info().Msgf("WAITING FOR SIGINT :)")
	<-sigint
	cancel(errors.New("server closed"))
	wg.Wait()
	log.Info().Msgf("Bye bye")
}

func enableUI(_ context.Context, router *httprouter.Router) {
	router.ServeFiles("/ui/*filepath", http.Dir(config.GetString("ui.dir")))
}

func enableSystemdModule(ctx context.Context, router *httprouter.Router) {
	topic := notifier_impl.NewStandardTopic()

	integration := systemd.New(ctx, topic)
	httpIntegration := systemd.Http(integration)

	router.GET("/ws", httpIntegration.StreamUnit)
}

func enableSecretdModule(ctx context.Context, router *httprouter.Router) {
	ctx, err := raft_runner.RunReplica[any](
		ctx,
		"secretd-v1",
		content_chraft.New(
			nil,
			content_chraft.TableConfig{Name: "secretd_secret", RefSize: 1, IncrementalID: true, IncrementalIDGetLimit: 5},
			content_chraft.TableConfig{Name: "secretd_env", RefSize: 1, IncrementalID: true, IncrementalIDGetLimit: 5},
		),
	)
	if err != nil {
		log.Panic().Msgf("failed to run secretd raft: %v", err)
	}

	secretStore := content_chraft.NewStorageClient(ctx, "secretd_secret")
	secretUsecase = mycontent_base.New[*entity.Secret](secretStore, 1)
	secretHandler := mycontentapi.New(
		secretUsecase,
		publicBaseURL+"/secretd/secret",
		[]string{"service"},
	)

	envStore := content_chraft.NewStorageClient(ctx, "secretd_env")
	envUsecase = mycontent_base.New[*entity.Env](envStore, 1)
	envHandler := mycontentapi.New(
		envUsecase,
		publicBaseURL+"/secretd/env",
		[]string{"service"},
	)

	// obviously right now is not secure
	router.POST("/secretd/secret", secretHandler.Post)
	router.GET("/secretd/secret", secretHandler.Get)
	router.DELETE("/secretd/secret", secretHandler.Delete)

	router.POST("/secretd/env", envHandler.Post)
	router.GET("/secretd/env", envHandler.Get)
	router.DELETE("/secretd/env", envHandler.Delete)
}

func getReplicaID() uint64 {
	replicaID, err := strconv.ParseUint(os.Getenv("DEPLOYD_REPLICA_ID"), 10, 64)
	if err != nil {
		log.Fatal().Msgf("invalid DEPLOYD_REPLICA_ID env value: %v", os.Getenv("DEPLOYD_REPLICA_ID"))
	}
	return replicaID
}

func mustEnv(env string) string {
	value := os.Getenv(env)
	if value == "" {
		log.Fatal().Msgf("invalid '%v' env value: %v", env, value)
	}

	return value
}

func enableDeploydModule(ctx context.Context, router *httprouter.Router) {
	ctx, err := raft_runner.RunReplica[any](
		ctx,
		"deployd-v1",
		content_chraft.New(nil,
			content_chraft.TableConfig{Name: "deployd_host", RefSize: 0},
			content_chraft.TableConfig{Name: "deployd_service", RefSize: 0},
			content_chraft.TableConfig{Name: "deployd_raft_host", RefSize: 1},
			content_chraft.TableConfig{Name: "deployd_raft_replica", RefSize: 1},
		),
	)
	if err != nil {
		log.Panic().Msgf("failed to run deployd raft: %v", err)
	}

	// config storage
	hostConfigStore := content_chraft.NewStorageClient(ctx, "deployd_host")
	hostConfigUsecase = mycontent_base.New[*entity.Host](hostConfigStore, 0)
	hostConfigHandler := mycontentapi.New(
		hostConfigUsecase,
		publicBaseURL+"/deployd/host",
		nil,
	)

	serviceDefinitionStorage := content_chraft.NewStorageClient(ctx, "deployd_service")
	serviceDefinitionUsecase = mycontent_base.New[*entity.ServiceDefinition](serviceDefinitionStorage, 0)
	serviceDefinitionHandler := mycontentapi.New(
		serviceDefinitionUsecase,
		publicBaseURL+"/deployd/service",
		nil,
	)

	raftHostHandler := mycontentapi.NewFromStorage[*entity.RaftHost](
		publicBaseURL+"/deployd/raft/host",
		[]string{"service"},
		content_chraft.NewStorageClient(ctx, "deployd_raft_host"),
		1,
	)

	raftReplicaHandler := mycontentapi.NewFromStorage[*entity.RaftReplica](
		publicBaseURL+"/deployd/raft/replica",
		[]string{"service"},
		content_chraft.NewStorageClient(ctx, "deployd_raft_replica"),
		1,
	)

	// Deployd raft host specific configuration
	router.POST("/deployd/host", hostConfigHandler.Post)
	router.GET("/deployd/host", hostConfigHandler.Get)
	router.DELETE("/deployd/host", hostConfigHandler.Delete)

	// Service registry
	router.POST("/deployd/service", serviceDefinitionHandler.Post)
	router.GET("/deployd/service", serviceDefinitionHandler.Get)
	router.DELETE("/deployd/service", serviceDefinitionHandler.Delete)

	// Replica registry for each service.
	// We get the source of truth from application that use deployd library.
	router.POST("/deployd/raft/replica", raftReplicaHandler.Post)
	router.GET("/deployd/raft/replica", raftReplicaHandler.Get)
	router.DELETE("/deployd/raft/replica", raftReplicaHandler.Delete)

	// Deployd raft host specific configuration
	router.POST("/deployd/raft/host", raftHostHandler.Post)
	router.GET("/deployd/raft/host", raftHostHandler.Get)
	router.DELETE("/deployd/raft/host", raftHostHandler.Delete)
}

func initWithTestData(
	ctx context.Context,
) error {
	var err error

	// Populate host config API with config from file
	// Only this config is required. The rest are temporary for debugging
	for {
		_, err = hostConfigUsecase.Post(ctx, &entity.Host{
			Ns:           "deployd",
			Host:         mustEnv("DEPLOYD_HOSTNAME"),
			OS:           mustEnv("DEPLOYD_OS"),
			Architecture: mustEnv("DEPLOYD_ARCH"),
			RaftConfig: entity.DeploydRaftConfig{
				ReplicaID: getReplicaID(),
			},
			FQDN:        "com.deployd",
			PublishedAt: time.Now(),
		}, nil)
		if err == nil {
			break
		}
		if !errors.Is(err, content_chraft.ErrNotReady) {
			return err
		}

		time.Sleep(1 * time.Second)
	}

	// init with one service (a "user-profile" simple app)
	_, err = serviceDefinitionUsecase.Post(ctx, &entity.ServiceDefinition{
		Ns:   "deployd",
		Id:   "user-profile",
		Name: "DG User Profile Service",
		Repository: entity.ArtifactdRepository{
			URL: "",
			Ns:  "deployd",
			ID:  "user-profile",
		},
		PublishedAt: time.Now(),
	}, nil)
	if err != nil {
		return err
	}

	// init with one repository (a "user-profile" simple app)
	_, err = repositoryUsecase.Post(ctx, &entity.Repository{
		Ns:          "deployd",
		Id:          "user-profile",
		Name:        "DG User Profile Repository",
		Source:      "https://github.com/desain-gratis/common",
		URLx:        "",
		PublishedAt: time.Now(),
	}, nil)
	if err != nil {
		return err
	}

	// init with one repository (a "user-profile" simple app)
	_, err = envUsecase.Post(ctx, &entity.Env{
		KV: entity.KV{
			Ns:      "deployd",
			Service: "user-profile",
			Value: map[string]string{
				"MESSAGE":           "Hello from deployd 👋👋👋",
				"DEPLOYD_RAFT_PORT": "9966",
			},
			PublishedAt: time.Now(),
		},
	}, nil)
	if err != nil {
		return err
	}

	check := &entity.Secret{
		KV: entity.KV{
			Ns:      "deployd",
			Service: "user-profile",
			Value: map[string]string{
				"signing-key": "obviously-not S0 s3Cure secret!",
				"api1.secret": "secret for api1",
				"api1.id":     "id for api1",
			},
			PublishedAt: time.Now(),
		},
	}
	// init with one repository (a "user-profile" simple app)
	_, err = secretUsecase.Post(ctx, check, nil)
	if err != nil {
		return err
	}

	return nil
}

// enableArtifactDienableArtifactdModulescoveryModule enables upload artifact discovery / metadata query
func enableArtifactdModule(ctx context.Context, router *httprouter.Router) {
	// TODO: separate between config / CRUD with the incrementalID so we can reset the DB more easily
	// TODO: we can use commit ID purely on the archive, need to modify the gh-action to use bare archive client (instead of Builder)
	ctx, err := raft_runner.RunReplica[any](
		ctx,
		"artifactd-v1",
		content_chraft.New(
			nil,
			content_chraft.TableConfig{Name: "artifactd_repository", RefSize: 0},
			content_chraft.TableConfig{Name: "artifactd_build", RefSize: 1, IncrementalID: true},
			content_chraft.TableConfig{Name: "artifactd_archive", RefSize: 2},
		),
	)
	if err != nil {
		log.Panic().Msgf("failed to run artifactd raft: %v", err)
	}

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

	repositoryStorage := content_chraft.NewStorageClient(ctx, "artifactd_repository")
	repositoryUsecase = mycontent_base.New[*entity.Repository](repositoryStorage, 0)

	repositoryHandler := mycontentapi.New(
		repositoryUsecase,
		publicBaseURL+"/artifactd/repository",
		nil,
	)

	buildHandler := mycontentapi.NewFromStorage[*entity.BuildArtifact](
		publicBaseURL+"/artifactd/build",
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
		publicBaseURL+"/artifactd/archive",
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
