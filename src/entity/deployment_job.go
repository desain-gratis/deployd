package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

type (
	DeploymentJobStatus      string
	HostRestartServiceStatus string
	HostConfigurationStatus  string
)

const (
	// Overall status for the deployment job
	DeploymentJobStatusQueued      DeploymentJobStatus = "QUEUED"
	DeploymentJobStatusConfiguring DeploymentJobStatus = "CONFIGURING"
	DeploymentJobStatusConfigured  DeploymentJobStatus = "CONFIGURED"
	DeploymentJobStatusDeploying   DeploymentJobStatus = "DEPLOYING"
	DeploymentJobStatusDeployed    DeploymentJobStatus = "DEPLOYED"
	DeploymentJobStatusSuccess     DeploymentJobStatus = "SUCCESS"
	DeploymentJobStatusCancelled   DeploymentJobStatus = "CANCELLED"
	DeploymentJobStatusTimeOut     DeploymentJobStatus = "TIMEOUT"
	DeploymentJobStatusFailed      DeploymentJobStatus = "FAILED"

	// Host / worker specific configuring step status
	HostConfigurationStatusPending     HostConfigurationStatus = "PENDING"
	HostConfigurationStatusConfiguring HostConfigurationStatus = "CONFIGURING"
	HostConfigurationStatusSuccess     HostConfigurationStatus = "SUCCESS"
	HostConfigurationStatusFailed      HostConfigurationStatus = "FAILED"
	HostConfigurationStatusCancelled   HostConfigurationStatus = "CANCELLED"
	HostConfigurationStatusTimeOut     HostConfigurationStatus = "TIMEOUT"

	// Host / worker specific deployment (restart service, routing, etc.) step status
	HostRestartServiceStatusPending        HostRestartServiceStatus = "PENDING"
	HostRestartServiceStatusStarting       HostRestartServiceStatus = "STARTING"        // run cloudflared again
	HostRestartServiceStatusDrainTraffic   HostRestartServiceStatus = "DRAIN_TRAFFIC"   // stop cloudflared and wait; for networked service
	HostRestartServiceStatusRestarting     HostRestartServiceStatus = "RESTARTING"      // stop service, update symlink, start (systemd); for raft service
	HostRestartServiceStatusWaitReady      HostRestartServiceStatus = "WAIT_READY"      // healthcheck endpoint that includes raft get leader
	HostRestartServiceStatusRoutingTraffic HostRestartServiceStatus = "ROUTING_TRAFFIC" // run cloudflared again
	HostRestartServiceStatusSuccess        HostRestartServiceStatus = "SUCCESS"
	HostRestartServiceStatusFailed         HostRestartServiceStatus = "FAILED"
	HostRestartServiceStatusTimeOut        HostRestartServiceStatus = "TIMEOUT"
)

type HostDeploymentJob struct {
	*DeploymentJob
	Status HostRestartServiceStatus `json:"status"`
}

// Limitation of DG framework; if you have an entity that can indexed many way, you need to make separate struct
type DeploymentJobByService struct {
	*DeploymentJob
}

func (d *DeploymentJobByService) RefIDs() []string {
	return nil
}

type DeploymentJob struct {
	Ns string `json:"namespace"`
	Id string `json:"id"`

	// main status / DAG
	Status            DeploymentJobStatus `json:"status"`
	RestartServiceJob RestartServiceJob   `json:"restart_service_job"`
	ConfigureHostJob  ConfigureHostJob    `json:"configure_host_job"`

	// The request; when displaying can be omitted
	Request *SubmitDeploymentJobRequest `json:"request,omitempty"`

	// The target host in which we will deploy our service to;
	// In dragonboat lingo, this would be the Nodes, as opossed to NonVotings, Witnesses, Removed member type.
	// able to be omitted for display
	Target []Host `json:"target,omitempty"`

	RaftConfig *RaftConfig `json:"raft_config,omitempty"`

	PendingUserAction *PendingUserAction `json:"pending_user_action,omitempty"`

	Url         string     `json:"url"`
	PublishedAt time.Time  `json:"published_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

type RaftConfig struct {
	Service map[string]RaftServiceConfig `json:"service,omitempty"`
	Shards  map[uint64]RaftShardConfig   `json:"shards,omitempty"`
}

// For front end to render the CTA, if "believe"(aka.auto confirm) is not specified
type PendingUserAction struct {
	// after configuration finish
	ConfirmDeployment  *ConfirmDeployment  `json:"confirm_deployment,omitempty"`
	ContinueDeployment *ContinueDeployment `json:"continue_deployment,omitempty"`
}

type ConfirmDeployment struct {
	Message        string
	CTAButtonLabel string
}

type ContinueDeployment struct {
	Message    string
	TargetHost string
}

type ConfigureHostJob struct {
	Status map[string]HostConfigurationState `json:"status"`
}

type RestartServiceJob struct {
	ConfirmedBy  string                             `json:"confirmed_by,omitempty"`
	CurrentOrder *uint                              `json:"current_order,omitempty"`
	HostOrder    []string                           `json:"host_order"`
	Status       map[string]HostRestartServiceState `json:"status"`
	ConfirmedAt  *time.Time                         `json:"confirmed_at,omitempty"`
}

type HostRestartServiceState struct {
	ErrorMessage *string                  `json:"error_message,omitempty"`
	Status       HostRestartServiceStatus `json:"status"`
	URL          string                   `json:"url,omitempty"` // url with job id to stream log from
	JobID        string                   `json:"job_id,omitempty"`
}

type HostConfigurationState struct {
	ErrorMessage *string                 `json:"error_message,omitempty"`
	Status       HostConfigurationStatus `json:"status"`
	URL          string                  `json:"url,omitempty"` // url with job id to stream log from
	JobID        string                  `json:"job_id,omitempty"`
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
