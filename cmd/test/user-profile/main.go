package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	mycontentapi "github.com/desain-gratis/common/delivery/mycontent-api"
	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	content_chraft "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/clickhouse-raft"
	raftr "github.com/desain-gratis/common/lib/raft/runner"
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
		fmt.Fprint(w, "Latest! 💕💕💕💝🥰❤️❤️🌴🌴🌴🧨😘\n")
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

	server := &http.Server{
		Addr:    "0.0.0.0:10001",
		Handler: router,
	}

	log.Info().Msgf("Baruu. User profile service is running at http://0.0.0.0:10001")

	server.ListenAndServe()
}
