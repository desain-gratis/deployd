package deployjob

import (
	"context"
	"log/slog"
	"os"

	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	"github.com/desain-gratis/deployd/src/entity"

	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
)

// Dependencies in the integration side (not inside raft)
type Dependencies struct {
	// host specific configuration
	HostConfigUsecase *mycontent_base.Handler[*entity.Host]

	// service configuration (to be installed on the host)
	ServiceDefinitionUsecase *mycontent_base.Handler[*entity.ServiceDefinition]

	// service instance for each host
	ServiceDeploymentUsecase *mycontent_base.Handler[*entity.ServiceInstanceHost]

	// archive / artifact repository storing build & archive information
	RepositoryUsecase *mycontent_base.Handler[*entity.Repository]

	// store service's env
	EnvUsecase *mycontent_base.Handler[*entity.Env]

	// store service's secret
	SecretUsecase *mycontent_base.Handler[*entity.Secret]

	JobUsecase *mycontent_base.Handler[*entity.DeploymentJob]

	// attachment
	BuildArtifactUsecase *mycontent_base.HandlerWithAttachment

	// deploy job client
	RaftJobUsecase *deployjob.Client
}

// or interface
type integration struct {
	// Http interface for the whole deployment jobs
	Http *httpHandler

	// In process interface exposed for consuming events;
	Event *eventHandler
}

func New(ctx context.Context, deps *Dependencies, host *entity.Host) *integration {
	jobsController := &jobsController{
		dependencies:      deps,
		host:              host,
		deploymentJobPool: make(map[string]*deploymentJob),
		log: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})).
			With("type", "controller").
			With("job", "deployment-controller"),
	}

	i := &integration{
		Http:  &httpHandler{jobsController: jobsController, dependencies: deps},
		Event: &eventHandler{jobsController: jobsController, dependencies: deps},
	}

	return i
}
