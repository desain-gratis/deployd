package deployjob

import (
	"time"

	"github.com/desain-gratis/deployd/src/entity"
)

type (
	EventDeploymentJobCreated SubmitJobResponse

	EventAllHostConfigured       ConfigurationUpdateResponse
	EventHostConfigured          ConfigurationUpdateResponse
	EventHostConfigurationUpdate ConfigurationUpdateResponse

	EventServiceRestarted     HostRestartServiceUpdateResponse
	EventServiceRestartUpdate HostRestartServiceUpdateResponse
	EventAllServiceRestarted  HostRestartServiceUpdateResponse

	EventDeploymentJobFailed  CommonResponse
	EventDeploymentJobSuccess CommonResponse

	// Lets go deploy
	EventRestartConfirmed HostRestartConfirmationResponse

	EventDeploymentJobCancelled struct {
		Job entity.DeploymentJob
		// can add other event messages..
	}
)

type CancelJobRequest struct {
	Ns      string `json:"namespace"`
	JobId   string `json:"job_id"`
	Service string `json:"service"`

	Reason string `json:"reason"`

	// todo: actor

	IsBelieve bool      `json:"is_believe"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

type ConfigurationUpdateRequest struct {
	Ns      string `json:"namespace"`
	JobId   string `json:"job_id"`
	Service string `json:"service"`

	HostName     string                         `json:"host_name"`
	Status       entity.HostConfigurationStatus `json:"status"`
	ErrorMessage *string                        `json:"error_message,omitempty"`

	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConfigurationUpdateResponse CommonResponse

type RestartConfirmation struct {
	Ns      string `json:"namespace"`
	JobId   string `json:"job_id"`
	Service string `json:"service"`

	Host    string `json:"host"`
	Message string `json:"message"`

	Agent string `json:"agent"` // who confirm it

	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

type HostRestartServiceUpdateResponse struct {
	CommonResponse
	Step int `json:"current_step"`
}

type CommonResponse struct {
	TriggerHost string               `json:"trigger_host"`
	Job         entity.DeploymentJob `json:"job"`
	Failed      bool                 `json:"failed,omitempty"`
	FailReason  *string              `json:"fail_reason,omitempty"`
}

type HostRestartConfirmationResponse struct {
	Step        int                  `json:"current_step"`
	TargetHost  string               `json:"target_host"`
	Job         entity.DeploymentJob `json:"job"`
	TriggerHost string               `json:"trigger_host"`
	Message     string               `json:"message"`
}

type HostRestartServiceUpdateRequest struct {
	Ns      string `json:"namespace"`
	JobId   string `json:"job_id"`
	Service string `json:"service"`

	HostName     string                          `json:"host_name"`
	Status       entity.HostRestartServiceStatus `json:"status"`
	ErrorMessage *string                         `json:"message,omitempty"`

	Order *int `json:"order"`

	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updated_at"`
}
