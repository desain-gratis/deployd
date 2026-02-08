package deployjob

import (
	"time"

	"github.com/desain-gratis/deployd/src/entity"
)

type EventDeploymentJobCreated struct {
	Job entity.DeploymentJob
	// can add event messages.. etc..
}

type EventDeploymentJobCancelled struct {
	Job entity.DeploymentJob
	// can add other event messages..
}

type CancelJobRequest struct {
	Ns      string `json:"namespace"`
	Id      string `json:"id"`
	Service string `json:"service"`

	Reason string `json:"reason"`

	// todo: actor

	IsBelieve bool      `json:"is_believe"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

type ConfigurationUpdateRequest struct {
	Ns      string `json:"namespace"`
	Id      string `json:"id"`
	Service string `json:"service"`

	HostName string                     `json:"host_name"`
	Status   entity.DeploymentJobStatus `json:"status"`
	Message  string                     `json:"message"`

	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

type DeployConfirmation struct {
	Ns      string `json:"namespace"`
	Id      string `json:"id"`
	Service string `json:"service"`

	Status  string `json:"status"`
	Message string `json:"string"`

	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

type DeploymentUpdateRequest struct {
	Ns      string `json:"namespace"`
	Id      string `json:"id"`
	Service string `json:"service"`

	HostName string                     `json:"host_name"`
	Status   entity.DeploymentJobStatus `json:"status"`
	Message  string                     `json:"message"`

	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}
