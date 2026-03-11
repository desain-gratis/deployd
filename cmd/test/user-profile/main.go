package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "net/http/pprof"

	mycontentapi "github.com/desain-gratis/common/delivery/mycontent-api"
	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	content_chraft "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/clickhouse-raft"
	raftr "github.com/desain-gratis/common/lib/raft/runner"
	"github.com/desain-gratis/deployd/internal/src/raft-app/hello"
	"github.com/desain-gratis/deployd/src/deployd"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router := httprouter.New()

	router.GET("/", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		fmt.Fprint(w, "🥰❤️💕🌞🥳🌞💕☀️👍💝🧨😘🌴💫😘☺️😌✅✅✅✅❤️❤️❤️😛👍🤪✅✅✅✅😆🤣🥰🥰😘😋😁☺️😁😁🥳✅✅ LGTM!!! \n")
		key := "MESSAGE"
		fmt.Fprintf(w, "Env read: %v = %v.\n", key, os.Getenv(key))
		fmt.Fprintf(w, "Env read: %v = %v.\n", "DEPLOYD_SERVICE_BUILD_COMMIT_ID", os.Getenv("DEPLOYD_SERVICE_BUILD_COMMIT_ID"))
		fmt.Fprintf(w, "Env read: %v = %v.\n", "DEPLOYD_SERVICE_BUILD_ID", os.Getenv("DEPLOYD_SERVICE_BUILD_ID"))
		fmt.Fprintf(w, "Env read: %v = %v.\n", "DEPLOYD_SERVICE_BUILD_DATE", os.Getenv("DEPLOYD_SERVICE_BUILD_DATE"))
		fmt.Fprintf(w, "Env read: %v = %v.\n", "DEPLOYD_SERVICE_BUILD_TAG", os.Getenv("DEPLOYD_SERVICE_BUILD_TAG"))
	})

	config := viper.New()
	err := deployd.InjectSecretToViper(config)

	err = raftr.InitWithConfigFile(os.Getenv("DEPLOYD_RAFT"))
	if err != nil {
		log.Fatal().Msgf("err raft: %v", err)
	}

	raftr.WithClickhouseStorage(
		config.GetString("storage.clickhouse.replica.address"),
		config.GetString("storage.clickhouse.replica.username"),
		config.GetString("storage.clickhouse.replica.password"),
		fmt.Sprintf("user-profile-%v", os.Getenv("DEPLOYD_HOST_REPLICA_ID")),
	)

	ctx, err = raftr.RunReplica[any](
		ctx,
		"user-profile-v1",
		content_chraft.New(
			content_chraft.TableConfig{Name: "user_profile", RefSize: 0},
		),
	)
	if err != nil {
		log.Panic().Msgf("failed to run secretd raft: %v", err)
	}

	userProfileStore := content_chraft.NewStorageClient(ctx, "user_profile")
	userProfileUsecase := mycontent_base.New[*UserProfile](userProfileStore, 0)
	userProfileHandler := mycontentapi.New(
		userProfileUsecase,
		"/profile",
		nil,
	)

	router.POST("/profile", userProfileHandler.Post)
	router.GET("/profile", userProfileHandler.Get)
	router.DELETE("/profile", userProfileHandler.Delete)

	ctxHello, err := raftr.RunReplica[any](
		ctx,
		"hello-world-v1",
		hello.New(),
	)
	if err != nil {
		log.Panic().Msgf("failed to run secretd raft: %v", err)
	}

	helloClient := hello.NewClient(ctxHello)

	router.POST("/hello", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		rc := http.MaxBytesReader(w, r.Body, 1024*1024)
		payload, err := io.ReadAll(rc)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error: %v", err)
			return
		}

		greetings, err := helloClient.GetGreeting(ctx, payload, true)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error: %v", err)
			return
		}
		fmt.Fprintf(w, "success: %v", greetings)
	})

	router.POST("/hello-async", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		rc := http.MaxBytesReader(w, r.Body, 1024*1024)
		payload, err := io.ReadAll(rc)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error: %v", err)
			return
		}

		greetings, err := helloClient.GetGreeting(ctx, payload, false)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error: %v", err)
			return
		}
		fmt.Fprintf(w, "success: %v", greetings)
	})

	router.Handler("GET", "/debug/pprof/*pprof", http.DefaultServeMux)

	server := &http.Server{
		Addr:    "0.0.0.0:10001",
		Handler: router,
	}

	log.Info().Msgf("Baruu. User profile service is running at http://0.0.0.0:10001")

	go server.ListenAndServe()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	log.Info().Msgf("Server is running.")
	<-sigint
	ctxt, cancelt := context.WithTimeout(ctx, 30*time.Second)
	defer cancelt()
	_ = server.Shutdown(ctxt)
	log.Info().Msgf("Closing raft..")
	raftr.Close()
	log.Info().Msgf("Raft cosed..")
	log.Info().Msgf("Bye bye")
}
