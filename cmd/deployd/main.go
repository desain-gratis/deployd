package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	mycontentapi "github.com/desain-gratis/common/delivery/mycontent-api"
	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	blob_s3 "github.com/desain-gratis/common/delivery/mycontent-api/storage/blob/s3"
	content_badgerraft "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/badger-raft"

	content_chraft "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/clickhouse-raft"
	"github.com/desain-gratis/common/lib/notifier"
	notifier_api "github.com/desain-gratis/common/lib/notifier/api"
	notifier_impl "github.com/desain-gratis/common/lib/notifier/impl"

	// raftr "github.com/desain-gratis/common/lib/raft/runner"
	runneretcd "github.com/desain-gratis/common/lib/raft/runner-etcd"
	deployjobintegration "github.com/desain-gratis/deployd/internal/src/deploy-job"
	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
	"github.com/desain-gratis/deployd/internal/src/systemd"
	"github.com/desain-gratis/deployd/src/deployd"
	"github.com/desain-gratis/deployd/src/entity"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Logger()
}

var (
	publicBaseURL string

	currentHost *entity.Host

	// host specific configuration
	hostConfigUsecase *mycontent_base.Handler[*entity.Host]

	// service configuration (to be installed on the host)
	serviceDefinitionUsecase *mycontent_base.Handler[*entity.ServiceDefinition]

	// archive / artifact repository storing build & archive information
	repositoryUsecase *mycontent_base.Handler[*entity.Repository]

	buildUsecase *mycontent_base.VersionedHandler[*entity.BuildArtifact]

	// store service's env
	envUsecase *mycontent_base.Handler[*entity.Env]

	// store service's secret
	secretUsecase *mycontent_base.Handler[*entity.Secret]

	// store service's routing
	routingUsecase *mycontent_base.Handler[*entity.Routing]

	jobUsecase *mycontent_base.VersionedHandler[*entity.DeploymentJob] // todo view only versioned

	buildArtifactUsecase *mycontent_base.HandlerWithAttachment

	// deploy job client
	raftDeployjobUsecase *deployjob.Client

	deploydTopic notifier.Topic
)

func main() {
	ctx, cancel := context.WithCancelCause(context.Background())

	initConfig()

	currentHost = &entity.Host{
		Ns:           "deployd",
		Host:         config.GetString("host.name"),
		OS:           config.GetString("host.os"),
		Architecture: config.GetString("host.architecture"),
		FQDN:         config.GetString("http.public.fqdn"),
		RaftConfig: entity.RaftHostConfig{
			ReplicaID:       config.GetUint64("raft.replica_id"),
			BaseWALDir:      config.GetString("raft.base_wal_dir"),
			BaseNodeHostDir: config.GetString("raft.base_node_host_dir"),
			RTTMillisecond:  900,
		},
		PublishedAt: time.Now(),
	}

	err := deployd.InjectSecretToViper(config.Viper)
	if err != nil && !errors.Is(err, deployd.ErrNotConfigured) {
		log.Panic().Msgf("failed to merge config with secret: %v", err)
	} else if err != nil {
		log.Warn().Msgf("not in deployd environment")
	}

	// All event regarding job lifecycle will be published to this topic
	deploydTopic = notifier_impl.NewStandardTopic()

	// err = raftr.InitWithConfigFile(os.Getenv("DEPLOYD_RAFT"))
	// if err != nil {
	// 	log.Panic().Msgf("failed to init raft: %v", err)
	// }

	// test in memory first
	opts := badger.DefaultOptions("").WithInMemory(true)
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal().Msgf("UHUY", err)
	}

	// A raft application that provides distributed mycontent storage
	// TODO: have better API
	badgerStorageApp := content_badgerraft.New(
		db,
		// artifactd module
		content_badgerraft.TableConfig{Name: "artifactd__repository", RefSize: 0},
		content_badgerraft.TableConfig{Name: "artifactd__build", RefSize: 1, Versioned: true},
		content_badgerraft.TableConfig{Name: "artifactd__archive", RefSize: 2},

		// deployd module
		content_badgerraft.TableConfig{Name: "deployd__host", RefSize: 0},
		content_badgerraft.TableConfig{Name: "deployd__service", RefSize: 0},
		content_badgerraft.TableConfig{Name: "deployd__raft_host", RefSize: 1},
		content_badgerraft.TableConfig{Name: "deployd__raft_replica", RefSize: 1},

		// secretd module
		content_badgerraft.TableConfig{Name: "secretd__secret", RefSize: 0, Versioned: true, VersionedGetLimit: 5},
		content_badgerraft.TableConfig{Name: "secretd__env", RefSize: 0, Versioned: true, VersionedGetLimit: 5},
		content_badgerraft.TableConfig{Name: "secretd__routing", RefSize: 0, Versioned: true, VersionedGetLimit: 5},
	)

	//
	// jobStorageApp := content_badgerraft.New(
	// 	db,
	// 	// artifactd module
	// 	content_badgerraft.TableConfig{Name: "artifactd__repository", RefSize: 0},
	// 	content_badgerraft.TableConfig{Name: "artifactd__build", RefSize: 0, Versioned: true},
	// 	content_badgerraft.TableConfig{Name: "artifactd__archive", RefSize: 2},
	// )

	// todo: refactor raftr API
	// raftr.WithClickhouseStorage(
	// 	config.GetString("storage.clickhouse.replica.address"),
	// 	config.GetString("storage.clickhouse.replica.username"),
	// 	config.GetString("storage.clickhouse.replica.password"),
	// 	fmt.Sprintf("deployd-%v", currentHost.RaftConfig.ReplicaID),
	// )

	wg := new(sync.WaitGroup)
	router := httprouter.New()

	// Run the raft app
	ctx, _, err = runneretcd.RunWithConfig("/etc/etcd-raft.yaml", "deployd", badgerStorageApp)
	if err != nil {
		log.Fatal().Msgf("err init raft: %v", err)
	}
	// This is all the place to store configuration (mycontent maxxing)
	// enableSystemdModule(ctx, router)
	enableArtifactdModule(ctx, router, badgerStorageApp)
	enableDeploydModule(ctx, router, badgerStorageApp)
	enableSecretdModule(ctx, router, badgerStorageApp)

	// This is our replicated raft state-machine app for deployment job.
	// It can also exposes mycontent datastore for easy access (read only).
	// All write command are managed by the application
	enableJobModule(ctx, router)

	enableUI(ctx, router)

	err = initHostInformation(ctx)
	if err != nil {
		log.Err(err).Msgf("failed to initialize host in deployd")
	}

	// update host config first

	go startRouter(ctx, wg, router, config.GetString("http.public.address"))

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	log.Info().Msgf("Server is running.")
	<-sigint
	cancel(errors.New("server closed"))
	wg.Wait()
	log.Info().Msgf("Closing raft")
	// raftr.Close()
	log.Info().Msgf("Bye bye")
}

func initHostInformation(ctx context.Context) error {
	var err error

	// Populate host config API with config from file
	// Only this config is required. The rest are temporary for debugging
	for {
		_, err = hostConfigUsecase.Post(ctx, currentHost, nil)
		if err == nil {
			break
		}
		if !errors.Is(err, content_chraft.ErrNotReady) {
			return err
		}

		time.Sleep(1 * time.Second)
	}

	return nil
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

func enableJobModule(ctx context.Context, router *httprouter.Router) {
	subscription, err := deploydTopic.Subscribe(ctx, notifier_impl.NewStandardSubscriber(nil))
	if err != nil {
		log.Fatal().Msgf("failed to run subscribe to a topic:  %v", err)
	}

	logSubscription, err := deploydTopic.Subscribe(ctx, notifier_impl.NewStandardSubscriber(nil))
	if err != nil {
		log.Fatal().Msgf("failed to run subscribe to a topic:  %v", err)
	}

	// IMPORTANT: start often forgotten. Start before replica started to make sure no message is lost
	subscription.Start()
	logSubscription.Start()

	go func() {
		for msg := range logSubscription.Listen() {
			msga, ok := msg.(deployjobintegration.Log)
			if !ok {
				continue
			}

			log.Info().Msgf("msg=%v", msga["msg"])
		}
	}()

	jobApp := deployjob.New(deploydTopic)

	ctx, _, err = runneretcd.RunWithConfig("/etc/etcd-raft.yaml", "job", jobApp)
	if err != nil {
		log.Fatal().Msgf("err init raft: %v", err)
	}

	// "ordinary" mycontent
	// notice we don't need repo, because it's managed inside the job state machine app, we only get the read only version
	jobUsecase = jobApp.GetJobStore()
	jobHandler := mycontentapi.New(
		jobUsecase,
		publicBaseURL+"/deployd/job",
		nil,
	)

	// store only the latest successful deployment job
	// not used anywhere other than this
	lastSuccessfulJobUsecase := jobApp.GetSuccessfulJobStore()
	lastSuccessfulJobHandler := mycontentapi.New(
		lastSuccessfulJobUsecase,
		publicBaseURL+"/deployd/job",
		nil,
	)

	raftDeployjobUsecase = deployjob.NewClient(ctx)

	integration := deployjobintegration.New(
		ctx,
		deploydTopic,
		&deployjobintegration.Dependencies{
			HostConfig: deployjobintegration.HostConfig{
				LocalClickhouseConfig: deployjobintegration.LocalClickhouseConfig{
					Address:  config.GetString("storage.clickhouse.replica.address"),
					Username: config.GetString("storage.clickhouse.replica.username"),
					Password: config.GetString("storage.clickhouse.replica.username"),
				},
			},
			HostConfigUsecase:        hostConfigUsecase,
			ServiceDefinitionUsecase: serviceDefinitionUsecase,
			RepositoryUsecase:        repositoryUsecase,
			EnvUsecase:               envUsecase,
			SecretUsecase:            secretUsecase,
			RaftJobUsecase:           raftDeployjobUsecase,
			BuildArtifactUsecase:     buildArtifactUsecase,
			JobUsecase:               jobUsecase,
			RoutingUsecase:           routingUsecase,
			BuildUsecase:             buildUsecase,
		},
		currentHost,
	)

	router.POST("/deployd/submit-job", integration.Http.SubmitJob)
	router.POST("/deployd/job/:service/:id/cancel", integration.Http.CancelJob)
	router.POST("/deployd/job/:service/:id/proceed", integration.Http.ConfirmDeployment)

	router.GET("/deployd/job", jobHandler.Get)
	router.GET("/deployd/successful-job", lastSuccessfulJobHandler.Get)

	integration.Event.StartConsumer(ctx, deploydTopic, subscription)

	handler := notifier_api.NewTopicAPI(deploydTopic)
	// websocket version
	wsWhitelist := []string{
		"http://localhost:*", "http://localhost",
		"http://mb1:*", "http://mb2:*", "http://mb3:*",
		"http://mb1", "http://mb2", "http://mb3",
		"http://hz1:*", "http://hz2:*", "http://hz3:*",
		"http://hz1", "http://hz2", "http://hz3",
	}

	// job read; non-websocket version
	router.GET("/deployd/job/stat", handler.Metrics)

	// real-time deploy job state update
	router.GET("/deployd/job/tail", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		reqNs := r.URL.Query().Get("namespace")
		srvId := r.URL.Query().Get("service")
		reqId := r.URL.Query().Get("id")
		handler.TailTransform(filterDeploymentJob(reqNs, srvId, reqId), transformDeployJobEvent)(w, r, p)
	})
	router.GET("/deployd/job/tail/ws", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		reqNs := r.URL.Query().Get("namespace")
		srvId := r.URL.Query().Get("service")
		reqId := r.URL.Query().Get("id")
		handler.Websocket(ctx, wsWhitelist, filterDeploymentJob(reqNs, srvId, reqId), transformDeployJobEvent)(w, r, p)
	})

	// real-time worker log and state update
	router.GET("/worker/deployment/tail", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		reqNs := r.URL.Query().Get("namespace")
		srvId := r.URL.Query().Get("service")
		reqId := r.URL.Query().Get("id")
		handler.TailTransform(filterWorkerLog(reqNs, srvId, reqId))(w, r, p)
	})
	router.GET("/worker/deployment/tail/ws", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		reqNs := r.URL.Query().Get("namespace")
		srvId := r.URL.Query().Get("service")
		reqId := r.URL.Query().Get("id")
		handler.Websocket(ctx, wsWhitelist, filterWorkerLog(reqNs, srvId, reqId))(w, r, p)
	})
}

func enableSecretdModule(ctx context.Context, router *httprouter.Router, raftStorage *content_badgerraft.BadgerRaftApp) {
	// ctx, err := raftr.RunReplica[any](
	// 	ctx,
	// 	"secretd-v1",
	// 	content_chraft.New(
	// 		// TODO: i've changed because the entity is changed. refsize 0 / refsize get automatically from the type ..?
	// 		content_chraft.TableConfig{Name: "secretd__secret", RefSize: 0, Versioned: true, VersionedGetLimit: 5},
	// 		content_chraft.TableConfig{Name: "secretd__env", RefSize: 0, Versioned: true, VersionedGetLimit: 5},
	// 		content_chraft.TableConfig{Name: "secretd__routing", RefSize: 0, Versioned: true, VersionedGetLimit: 5},
	// 	),
	// )
	// if err != nil {
	// 	log.Panic().Msgf("failed to run secretd raft: %v", err)
	// }

	// secretStore := content_chraft.NewStorageClient(ctx, "secretd__secret")
	secretStore, err := raftStorage.GetContentRepository(ctx, "secretd__secret")
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}

	secretUsecase = mycontent_base.New[*entity.Secret](secretStore)
	secretHandler := mycontentapi.New(
		secretUsecase,
		publicBaseURL+"/secretd/secret",
		nil,
	)

	// envStore := content_chraft.NewStorageClient(ctx, "secretd__env")
	envStore, err := raftStorage.GetContentRepository(ctx, "secretd__env")
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}

	envUsecase = mycontent_base.New[*entity.Env](envStore)
	envHandler := mycontentapi.New(
		envUsecase,
		publicBaseURL+"/secretd/env",
		nil,
	)

	// routingStore := content_chraft.NewStorageClient(ctx, "secretd__routing")
	routingStore, err := raftStorage.GetContentRepository(ctx, "secretd__routing")
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}

	routingUsecase = mycontent_base.New[*entity.Routing](routingStore)
	routingHandler := mycontentapi.New(
		routingUsecase,
		publicBaseURL+"/secretd/routing",
		nil,
	)

	// obviously right now is not secure
	router.POST("/secretd/secret", secretHandler.Post)
	router.GET("/secretd/secret", secretHandler.Get)
	router.DELETE("/secretd/secret", secretHandler.Delete)

	router.POST("/secretd/env", envHandler.Post)
	router.GET("/secretd/env", envHandler.Get)
	router.DELETE("/secretd/env", envHandler.Delete)

	router.POST("/secretd/routing", routingHandler.Post)
	router.GET("/secretd/routing", routingHandler.Get)
	router.DELETE("/secretd/routing", routingHandler.Delete)
}

func enableDeploydModule(ctx context.Context, router *httprouter.Router, raftStorage *content_badgerraft.BadgerRaftApp) {
	// ctx, err := raftr.RunReplica[any](
	// 	ctx,
	// 	"deployd-v1",
	// 	content_chraft.New(
	// 		content_chraft.TableConfig{Name: "deployd__host", RefSize: 0},
	// 		content_chraft.TableConfig{Name: "deployd__service", RefSize: 0},
	// 		content_chraft.TableConfig{Name: "deployd__raft_host", RefSize: 1},
	// 		content_chraft.TableConfig{Name: "deployd__raft_replica", RefSize: 1},
	// 	),
	// )
	// if err != nil {
	// 	log.Panic().Msgf("failed to run deployd raft: %v", err)
	// }

	// config storage
	// hostConfigStore := content_chraft.NewStorageClient(ctx, "deployd__host")
	hostConfigStore, err := raftStorage.GetContentRepository(ctx, "deployd__host")
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}

	hostConfigUsecase = mycontent_base.New[*entity.Host](hostConfigStore)
	hostConfigHandler := mycontentapi.New(
		hostConfigUsecase,
		publicBaseURL+"/deployd/host",
		nil,
	)

	// serviceDefinitionStorage := content_chraft.NewStorageClient(ctx, "deployd__service")
	serviceDefinitionStorage, err := raftStorage.GetContentRepository(ctx, "deployd__service")
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}

	serviceDefinitionUsecase = mycontent_base.New[*entity.ServiceDefinition](serviceDefinitionStorage)
	serviceDefinitionHandler := mycontentapi.New(
		serviceDefinitionUsecase,
		publicBaseURL+"/deployd/service",
		nil,
	)

	raftHostStore, err := raftStorage.GetContentRepository(ctx, "deployd__raft_host")
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}

	raftHostHandler := mycontentapi.NewFromStorage[*entity.RaftHost](
		publicBaseURL+"/deployd/raft/host",
		[]string{"service"},
		raftHostStore,
		1,
	)

	raftReplicaStore, err := raftStorage.GetContentRepository(ctx, "deployd__raft_replica")
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}

	raftReplicaHandler := mycontentapi.NewFromStorage[*entity.RaftReplica](
		publicBaseURL+"/deployd/raft/replica",
		[]string{"service"},
		raftReplicaStore,
		1,
	)

	// Deployd raft host specific configuration
	router.GET("/deployd/host", hostConfigHandler.Get)
	// router.POST("/deployd/host", hostConfigHandler.Post) 	// cannot be edited by user
	// router.DELETE("/deployd/host", hostConfigHandler.Delete)	// cannot be edited by user

	// Service registry
	router.GET("/deployd/service", serviceDefinitionHandler.Get)
	router.POST("/deployd/service", serviceDefinitionHandler.Post)
	router.DELETE("/deployd/service", serviceDefinitionHandler.Delete)

	// Service deployment
	// router.GET("/deployd/deployment", serviceDeploymentHandler.Get)
	// cannot be edited by user
	// router.POST("/deployd/deployment", serviceDeploymentHandler.Post)
	// router.DELETE("/deployd/deployment", serviceDeploymentHandler.Delete)

	// Replica registry for each service.
	// We get the source of truth from application that use deployd library.
	router.POST("/deployd/raft/replica", raftReplicaHandler.Post)
	router.GET("/deployd/raft/replica", raftReplicaHandler.Get)
	router.DELETE("/deployd/raft/replica", raftReplicaHandler.Delete)

	// Deployd raft host specific configuration
	router.POST("/deployd/raft/host", raftHostHandler.Post)
	router.GET("/deployd/raft/host", raftHostHandler.Get)
	router.DELETE("/deployd/raft/host", raftHostHandler.Delete)

	// Deployd deployment: service definition that has been deployed
	// Because it is modified by server, we will not expose the Post & Delete interface
	// router.GET("/deployd/deployment", raftHostHandler.Get)
}

// enableArtifactDienableArtifactdModulescoveryModule enables upload artifact discovery / metadata query
func enableArtifactdModule(ctx context.Context, router *httprouter.Router, raftStorage *content_badgerraft.BadgerRaftApp) {
	// TODO: separate between config / CRUD with the incrementalID so we can reset the DB more easily
	// TODO: we can use commit ID purely on the archive, need to modify the gh-action to use bare archive client (instead of Builder)
	// ctx, err := raftr.RunReplica[any](
	// 	ctx,
	// 	"artifactd-v1",
	// 	content_chraft.New(
	// 		content_chraft.TableConfig{Name: "artifactd__repository", RefSize: 0},
	// 		content_chraft.TableConfig{Name: "artifactd__build", RefSize: 0, Versioned: true},
	// 		content_chraft.TableConfig{Name: "artifactd__archive", RefSize: 2},
	// 	),
	// )
	// if err != nil {
	// 	log.Panic().Msgf("failed to run artifactd raft: %v", err)
	// }

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

	// repositoryStorage := content_chraft.NewStorageClient(ctx, "artifactd__repository")
	repositoryStorage, err := raftStorage.GetContentRepository(ctx, "artifactd__repository")
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}
	repositoryUsecase = mycontent_base.New[*entity.Repository](repositoryStorage)

	repositoryHandler := mycontentapi.New(
		repositoryUsecase,
		publicBaseURL+"/artifactd/repository",
		nil,
	)

	// buildStorage := content_chraft.NewStorageClient(ctx, "artifactd__build")
	// an auto increment repo
	buildStorage, err := raftStorage.GetVersionedContentRepository(ctx, "artifactd__build")
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}

	buildUsecase = mycontent_base.NewVersioned[*entity.BuildArtifact](buildStorage)

	buildHandler := mycontentapi.New(
		buildUsecase,
		publicBaseURL+"/artifactd/build",
		nil,
	)

	archiveStorage, err := raftStorage.GetContentRepository(ctx, "artifactd__archive")
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}

	buildArtifactUsecase = mycontent_base.NewAttachment(
		archiveStorage,
		buildArtifactBlob,
		false,
		"artifactd/archive",
	)
	archiveHandler := mycontentapi.NewAttachment(
		buildArtifactUsecase,
		publicBaseURL+"/artifactd/archive",
		[]string{"build"},
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

func filterDeploymentJob(reqNs, reqService, reqId string) func(any) bool {
	return func(msg any) bool {
		var ns, srv, id string
		switch t := msg.(type) {
		case deployjob.EventDeploymentJobCreated:
			ns, srv, id = t.Job.Ns, t.Job.Request.Service.Id, t.Job.Id
		case deployjob.EventHostConfigurationUpdate:
			ns, srv, id = t.Job.Ns, t.Job.Request.Service.Id, t.Job.Id
		case deployjob.EventHostConfigured:
			ns, srv, id = t.Job.Ns, t.Job.Request.Service.Id, t.Job.Id
		case deployjob.EventAllHostConfigured:
			ns, srv, id = t.Job.Ns, t.Job.Request.Service.Id, t.Job.Id
		case deployjob.EventRestartConfirmed:
			ns, srv, id = t.Job.Ns, t.Job.Request.Service.Id, t.Job.Id
		case deployjob.EventServiceRestarted:
			ns, srv, id = t.Job.Ns, t.Job.Request.Service.Id, t.Job.Id
		case deployjob.EventServiceRestartUpdate:
			ns, srv, id = t.Job.Ns, t.Job.Request.Service.Id, t.Job.Id
		case deployjob.EventAllServiceRestarted:
			ns, srv, id = t.Job.Ns, t.Job.Request.Service.Id, t.Job.Id
		case deployjob.EventDeploymentJobSuccess:
			ns, srv, id = t.Job.Ns, t.Job.Request.Service.Id, t.Job.Id
		case deployjob.EventDeploymentJobFailed:
			ns, srv, id = t.Job.Ns, t.Job.Request.Service.Id, t.Job.Id
		case deployjobintegration.Log:
			return true
		}

		idMatch := reqId == id
		if reqId == "" {
			idMatch = true
		}
		nsMatch := reqNs == ns
		if reqNs == "*" || reqNs == "" {
			nsMatch = true
		}
		srvMatch := reqService == srv
		if reqService == "" {
			srvMatch = true
		}

		return !(nsMatch && idMatch && srvMatch)
	}
}

func filterWorkerLog(reqNs, reqSrv, reqId string) func(any) bool {
	return func(msg any) bool {
		lg, ok := msg.(deployjobintegration.Log)
		if !ok {
			return true
		}

		idMatch := reqId == fmt.Sprintf("%v", lg["job_id"])
		if reqId == "" {
			idMatch = true
		}
		nsMatch := reqNs == lg["namespace"]
		if reqNs == "*" || reqNs == "" {
			nsMatch = true
		}

		srvMatch := reqSrv == lg["service"]
		if reqSrv == "" {
			srvMatch = true
		}

		return !(nsMatch && idMatch && srvMatch)
	}
}

func transformDeployJobEvent(msg any) any {
	var eventName string
	var job *entity.DeploymentJob

	switch value := msg.(type) {
	case deployjob.EventDeploymentJobFailed:
		eventName = "deployment-job-update"
		job = &value.Job
	case deployjob.EventDeploymentJobCreated:
		eventName = "deployment-job-update"
		job = &value.Job
	case deployjob.EventHostConfigurationUpdate:
		eventName = "deployment-job-update"
		job = &value.Job
	case deployjob.EventHostConfigured:
		eventName = "deployment-job-update"
		job = &value.Job
	case deployjob.EventRestartConfirmed:
		eventName = "deployment-job-update"
		job = &value.Job
	case deployjob.EventServiceRestarted:
		eventName = "deployment-job-update"
		job = &value.Job
	case deployjob.EventServiceRestartUpdate:
		eventName = "deployment-job-update"
		job = &value.Job
	case deployjob.EventAllHostConfigured:
		eventName = "deployment-job-update"
		job = &value.Job
	case deployjob.EventAllServiceRestarted:
		eventName = "deployment-job-update"
		job = &value.Job
	case deployjob.EventDeploymentJobSuccess:
		eventName = "deployment-job-update"
		job = &value.Job
	}

	if job == nil {
		return "-"
	}

	if job.Status != "DEPLOYED" {
		job.Target = nil
		job.RaftConfig = nil
		job.Request = nil
	}

	// make it fair so it's like the websocket
	return map[string]any{
		"table": eventName,
		"job":   job,
	}
}
