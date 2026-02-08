package main

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	common_entity "github.com/desain-gratis/common/types/entity"
	"github.com/desain-gratis/deployd/src/entity"
	"github.com/rs/zerolog/log"

	contentsync "github.com/desain-gratis/common/delivery/mycontent-api-client"
)

func main() {

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
			{Id: "linux/amd64", Url: "test.tar.gz"},
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
