package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	common_entity "github.com/desain-gratis/common/types/entity"
	"github.com/desain-gratis/deployd/src/entity"
	"github.com/rs/zerolog/log"

	contentsync "github.com/desain-gratis/common/delivery/mycontent-api-client"
	mycontentapiclient "github.com/desain-gratis/common/delivery/mycontent-api-client"
)

const (
	host = "http://localhost:9401"
)

func main() {
	ctx := context.Background()

	err := initWithTestData(ctx)
	if err != nil {
		log.Err(err).Msgf("failed to initialize starting data in deployd: %v", err)
	}

	// tagsStr := strings.Join(tags, ",")
	data := []*entity.BuildArtifact{{
		Id:           "", // important to left this one empty
		Ns:           "deployd",
		CommitID:     "heh3h3h3h3",
		Branch:       "iguana",
		Actor:        "banana",
		Tag:          "hey",
		Data:         json.RawMessage(`{"source": "script"}`),
		PublishedAt:  time.Now(),
		Source:       "deployd-script",
		RepositoryID: "user-profile",
		OsArch:       []string{"linux/amd64"}, // hardcode first
		URLx:         "",
		Name:         "user-profile",
		Archive: []*common_entity.File{
			{Id: "linux/amd64", Url: "user-profile.tar.gz"},
		},
	}}

	u, err := url.Parse("http://localhost:9401/artifactd/build")
	if err != nil {
		log.Fatal().Msgf("err: %v", err)
	}
	buildSync := contentsync.Builder[*entity.BuildArtifact](u, "repository").
		WithNamespace("*").
		WithData(data)

	// TODO: improve DevX it's a bit painful
	// because you can get parameter automatically from parents
	//

	// notice there is no "repository" here because the main entity Artifact already have "repository" ref.
	buildSync.
		WithFiles(getArchive, "../archive", "build")

	// upload metadata

	err = buildSync.Build().Execute(context.Background())
	if err != nil {
		log.Panic().Msgf("failed to execute: %v", err)
	}
}

func initWithTestData(
	ctx context.Context,
) error {
	var err error

	// Populate host config API with config from file
	// Only this config is required. The rest are temporary for debugging

	serviceDefinitionUsecase := mycontentapiclient.New[*entity.ServiceDefinition](http.DefaultClient, host+"/deployd/service", nil, "")
	repositoryUsecase := mycontentapiclient.New[*entity.Repository](http.DefaultClient, host+"/artifactd/repository", nil, "")
	envUsecase := mycontentapiclient.New[*entity.Env](http.DefaultClient, host+"/secretd/env", []string{"service"}, "")
	secretUsecase := mycontentapiclient.New[*entity.Secret](http.DefaultClient, host+"/secretd/secret", []string{"service"}, "")

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
		Description:    "Hello",
		ExecutablePath: "./user-profile",
		BoundAddresses: []entity.BoundAddress{
			{Host: "localhost", Port: 10001},
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

func getArchive(t []*entity.BuildArtifact) []contentsync.FileContext[*entity.BuildArtifact] {
	result := make([]contentsync.FileContext[*entity.BuildArtifact], 0)
	for i := range t {
		for j := range t[i].Archive {
			result = append(result, contentsync.FileContext[*entity.BuildArtifact]{
				Base: t[i], File: &t[i].Archive[j],
			})
		}
	}
	return result
}
