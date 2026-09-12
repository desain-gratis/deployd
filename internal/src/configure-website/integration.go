package configurewebsite

import (
	"context"
	"log/slog"

	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	"github.com/desain-gratis/common/lib/notifier"
	"github.com/desain-gratis/deployd/src/entity"

	configurewebsite "github.com/desain-gratis/deployd/internal/src/raft-app/configure-website"
	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
)

// Dependencies in the integration side (not inside raft)
type Dependencies struct {
	// host specific configuration
	HostConfigUsecase *mycontent_base.Handler[*entity.Host]

	// service configuration (to be installed on the host)
	ServiceDefinitionUsecase *mycontent_base.Handler[*entity.ServiceDefinition]

	// archive / artifact repository storing build & archive information
	RepositoryUsecase *mycontent_base.Handler[*entity.Repository]

	// these three should be the Versioned
	// store service's env
	EnvUsecase *mycontent_base.Handler[*entity.Env]
	// store service's secret
	SecretUsecase *mycontent_base.Handler[*entity.Secret]
	// routing service
	RoutingUsecase *mycontent_base.Handler[*entity.Routing]

	BuildUsecase *mycontent_base.Handler[*entity.BuildArtifact]

	JobUsecase *mycontent_base.Handler[*entity.DeploymentJob]

	// attachment
	BuildArtifactUsecase *mycontent_base.HandlerWithAttachment

	// deploy job client
	RaftJobUsecase *deployjob.Client

	// nginx unit job client
	RaftConfigureWebsite *configurewebsite.Client
}

// or interface
type integration struct {
	// Http interface for the whole deployment jobs
	Http *httpHandler

	// In process interface exposed for consuming events;
	Event *eventHandler
}

func New(ctx context.Context, out notifier.Topic, deps *Dependencies, host *entity.Host, nginxUnitConfigAddress string) *integration {
	logger := slog.New(slog.DiscardHandler)

	localWorker := &localWorker{
		nginxUnitConfigAddress: nginxUnitConfigAddress,

		dependencies: deps,
		host:         host,

		deploymentJobPool: make(map[string]*configureJob), // TODO: use more efficient pool
		log: logger.
			With("host", host.Host),
	}

	i := &integration{
		Http:  &httpHandler{localWorker: localWorker, dependencies: deps},
		Event: &eventHandler{localWorker: localWorker, dependencies: deps},
	}

	return i
}
