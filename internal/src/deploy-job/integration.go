package deployjob

import (
	"context"

	mycontent_base "github.com/desain-gratis/common/delivery/mycontent-api/mycontent/base"
	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
	"github.com/desain-gratis/deployd/src/entity"
)

// Dependencies in the integration side (not inside raft)
type Dependencies struct {
	// host specific configuration
	HostConfigUsecase *mycontent_base.Handler[*entity.Host]

	// service configuration (to be installed on the host)
	ServiceDefinitionUsecase *mycontent_base.Handler[*entity.ServiceDefinition]

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

type integration struct {
	state  *state
	Http   *httpHandler
	Worker *worker
}

func New(ctx context.Context, deps *Dependencies, host *entity.Host) *integration {
	st := &state{activeJobs: make(map[string]*activeJob), host: host}

	i := &integration{
		// "user" interface / receive user command, webhook, etcs..
		Http: &httpHandler{state: st, dependencies: deps},

		// "program" interface; job status update here
		Worker: &worker{state: st, dependencies: deps, host: host},
	}

	return i
}
