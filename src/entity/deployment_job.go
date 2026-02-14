package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

type (
	DeploymentJobStatus string
)

const (
	// Initial spawned
	DeploymentJobStatusQueued DeploymentJobStatus = "QUEUED"

	// Start configuring
	DeploymentJobStatusConfiguring DeploymentJobStatus = "CONFIGURING"

	// All host configured
	DeploymentJobStatusConfigured DeploymentJobStatus = "CONFIGURED"

	// Wait for each host to restart service (one by one)
	DeploymentJobStatusDeploying DeploymentJobStatus = "DEPLOYING"

	// All hosts finished restart service
	DeploymentJobStatusDeployed DeploymentJobStatus = "DEPLOYED"

	// Clean up & archived
	DeploymentJobStatusSuccess DeploymentJobStatus = "SUCCESS"

	// Cancelled
	DeploymentJobStatusCancelled DeploymentJobStatus = "CANCELLED"

	// Timeout
	DeploymentJobStatusTimeOut DeploymentJobStatus = "TIMEOUT"

	// Failed
	DeploymentJobStatusFailed DeploymentJobStatus = "FAILED"
)

type HostDeploymentJob struct {
	*DeploymentJob
	Status HostDeploymentStatus `json:"status"`
}

type DeploymentJob struct {
	Ns     string              `json:"namespace"`
	Status DeploymentJobStatus `json:"status"`
	Id     string              `json:"id"`

	// TODO: add more relevant permanent info
	// (non-permanent should be on the raft app / on memory)

	Request       SubmitDeploymentJobRequest `json:"request"`
	Deployment    Deployment                 `json:"deployment"`
	Configuration Configuration              `json:"configuration"`

	Url         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

type Configuration struct {
	Status map[string]HostConfigurationStatusInfo `json:"status"`
}

type Deployment struct {
	ConfirmedBy  string                              `json:"confirmed_by,omitempty"`
	CurrentOrder *uint                               `json:"current_order,omitempty"`
	HostOrder    []string                            `json:"host_order"`
	Status       map[string]HostDeploymentStatusInfo `json:"status"`
}

type HostDeploymentStatusInfo struct {
	ErrorMessage *string              `json:"error_message,omitempty"`
	Status       HostDeploymentStatus `json:"status"`
}

type HostConfigurationStatusInfo struct {
	ErrorMessage *string                 `json:"error_message,omitempty"`
	Status       HostConfigurationStatus `json:"status"`
}

type HostDeploymentStatus string
type HostConfigurationStatus string

const (
	// mostly for raft service; ordinary service can run on the same port; but for raft, since they lock the directory to a single process, we just restart
	// without parallel start / multiple instances

	HostDeploymentStatusPending        HostDeploymentStatus = "PENDING"
	HostDeploymentStatusStarting       HostDeploymentStatus = "STARTING"        // run cloudflared again
	HostDeploymentStatusDrainTraffic   HostDeploymentStatus = "DRAIN_TRAFFIC"   // stop cloudflared and wait; for networked service
	HostDeploymentStatusRestarting     HostDeploymentStatus = "RESTARTING"      // stop service, update symlink, start (systemd); for raft service
	HostDeploymentStatusWaitReady      HostDeploymentStatus = "WAIT_READY"      // healthcheck endpoint that includes raft get leader
	HostDeploymentStatusRoutingTraffic HostDeploymentStatus = "ROUTING_TRAFFIC" // run cloudflared again
	HostDeploymentStatusSuccess        HostDeploymentStatus = "SUCCESS"
	HostDeploymentStatusFailed         HostDeploymentStatus = "FAILED"
	HostDeploymentStatusTimeOut        HostDeploymentStatus = "TIMEOUT"

	HostConfigurationStatusPending     HostConfigurationStatus = "PENDING"
	HostConfigurationStatusConfiguring HostConfigurationStatus = "CONFIGURING"
	HostConfigurationStatusSuccess     HostConfigurationStatus = "SUCCESS"
	HostConfigurationStatusFailed      HostConfigurationStatus = "FAILED"
	HostConfigurationStatusCancelled   HostConfigurationStatus = "CANCELLED"
	HostConfigurationStatusTimeOut     HostConfigurationStatus = "TIMEOUT"
)

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
