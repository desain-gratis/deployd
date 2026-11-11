package deployjob

import (
	"context"
	"log/slog"

	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	"github.com/desain-gratis/common/lib/notifier"
	"github.com/desain-gratis/deployd/src/entity"

	configurewebsite "github.com/desain-gratis/deployd/internal/src/raft-app/configure-website"
	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
)

// Dependencies in the integration side (not inside raft) & configurations
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

	NginxUnitAddress *string
}

// or interface
type integration struct {
	// Http interface for the whole deployment jobs
	Http           *httpHandler // todo: rename
	WebsiteHandler *httpWebsiteHandler

	// In process interface exposed for consuming events;
	Worker *eventHandler
}

func New(ctx context.Context, out notifier.Topic, deps *Dependencies, host *entity.Host) *integration {
	logger := slog.New(NewNotifierLogger(out))

	localWorker := &localWorker{
		dependencies:      deps,
		host:              host,
		deploymentJobPool: make(map[string]*deploymentJob), // TODO: use more efficient pool
		log: logger.
			With("host", host.Host).
			With("job-type", "deploy_service"),
	}

	localWebWorker := &localWebsiteWorker{
		nginxUnitConfigAddress: deps.NginxUnitAddress,
		dependencies:           deps,
		host:                   host,
		log: logger.
			With("host", host.Host).
			With("job-type", "deploy_website"),
	}

	i := &integration{
		Http:           &httpHandler{localWorker: localWorker, dependencies: deps},
		WebsiteHandler: &httpWebsiteHandler{localWorker: localWorker, dependencies: deps},
		Worker:         &eventHandler{localWorker: localWorker, localWebWorker: localWebWorker, dependencies: deps},
	}

	return i
}
