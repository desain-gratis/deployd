package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

type DeploymentJobStatus string

const (
	// Initial spawned
	DeploymentJobStatusQueued DeploymentJobStatus = "QUEUED"

	// Start configuring
	DeploymentJobStatusConfiguring DeploymentJobStatus = "CONFIGURING"

	// All host
	DeploymentJobStatusConfigured DeploymentJobStatus = "CONFIGURED"

	// Wait for each host to restart service (one by one)
	DeploymentJobStatusDeploying DeploymentJobStatus = "DEPLOYING"

	// All hosts finished restart service
	DeploymentJobStatusDeployed DeploymentJobStatus = "DEPLOYED"

	// Clean up & archived
	DeploymentJobStatusFinished DeploymentJobStatus = "FINISHED"

	// Cancelled
	DeploymentJobStatusCancelled DeploymentJobStatus = "CANCELLED"

	// Invalid
	DeploymentJobStatusInvalid DeploymentJobStatus = "INVALID"

	// Failed
	DeploymentJobStatusFailed DeploymentJobStatus = "FAILED"
)

type DeploymentJob struct {
	Ns     string              `json:"namespace"`
	Status DeploymentJobStatus `json:"status"`
	Id     string              `json:"id"`

	// TODO: add more relevant permanent info
	// (non-permanent should be on the raft app / on memory)

	Request SubmitDeploymentJobRequest `json:"request"`

	Url         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

func (d *DeploymentJob) CreatedTime() time.Time {
	return d.PublishedAt
}

func (d *DeploymentJob) ID() string {
	return d.Id
}

func (d *DeploymentJob) Namespace() string {
	return d.Ns
}

func (d *DeploymentJob) RefIDs() []string {
	return []string{d.Request.Service.Id}
}

func (d *DeploymentJob) URL() string {
	return d.Url
}

func (d *DeploymentJob) Validate() error {
	// TODO: all need to be add validation eventually
	return nil
}

func (d *DeploymentJob) WithCreatedTime(t time.Time) mycontent.Data {
	d.PublishedAt = t
	return d
}

func (d *DeploymentJob) WithID(id string) mycontent.Data {
	d.Id = id
	return d
}

func (d *DeploymentJob) WithNamespace(id string) mycontent.Data {
	d.Ns = id
	return d
}

func (d *DeploymentJob) WithURL(url string) mycontent.Data {
	d.Url = url
	return d
}
