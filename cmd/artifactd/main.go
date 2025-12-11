package main

import (
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	mycontentapi "github.com/desain-gratis/common/delivery/mycontent-api"
	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	blob_s3 "github.com/desain-gratis/common/delivery/mycontent-api/storage/blob/s3"
	content_clickhouse "github.com/desain-gratis/common/delivery/mycontent-api/storage/content/clickhouse"
	"github.com/desain-gratis/deploy/internal/src/artifactd"
)

func main() {

}

var (
	baseURL = "localhost:9000"
)

// enableArtifactUploadModule enables a key-value store to store build artifact based on commit ID
func enableArtifactUploadModule(router *httprouter.Router) {
	var ch driver.Conn

	// storage for linux amd64 build artifact
	linuxAmd64Archive := content_clickhouse.New(ch, "linux_amd64", 0)
	linuxAmd64ArchiveBlob, err := blob_s3.New(
		"localhost:9000",
		"this1s4ccessXey",
		"S3cR3tthebestd0tcom",
		false,
		"linux_amd64",
		"http://localhost:9000/linux/amd64",
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
		[]string{"org_id", "profile_id"},
		"",
	)

	// <os>/<arch>
	router.GET("/linux/amd64", linuxAmd64Handler.Get)
	router.POST("/linux/amd64", linuxAmd64Handler.Upload)
	router.DELETE("/linux/amd64", linuxAmd64Handler.Delete)

	// can add for linux/arm64 here later..
}

// enableArtifactDiscoveryModule enables upload artifact discovery / metadata query
func enableArtifactDiscoveryModule(router *httprouter.Router) {
	var ch driver.Conn

	handler := artifactd.New(ch)
	httpHandler := artifactd.Http(handler)

	// Stream all latest commit
	router.GET("/ws", httpHandler.StreamAll)
}
