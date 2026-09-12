package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

type (
	JobConfigureWebsiteStatus string
)

func (j JobConfigureWebsiteStatus) IsTerminal() bool {
	switch j {
	case JobConfigureWebsiteStatusSuccess, JobConfigureWebsiteStatusTimeOut, JobConfigureWebsiteStatusCancelled, JobConfigureWebsiteStatusFailed:
		return true
	}
	return false
}

const (
	// Overall status for the deployment job
	JobConfigureWebsiteStatusQueued      JobConfigureWebsiteStatus = "QUEUED"
	JobConfigureWebsiteStatusConfiguring JobConfigureWebsiteStatus = "CONFIGURING"
	JobConfigureWebsiteStatusConfigured  JobConfigureWebsiteStatus = "CONFIGURED"
	JobConfigureWebsiteStatusDeploying   JobConfigureWebsiteStatus = "DEPLOYING"
	JobConfigureWebsiteStatusDeployed    JobConfigureWebsiteStatus = "DEPLOYED"
	JobConfigureWebsiteStatusSuccess     JobConfigureWebsiteStatus = "SUCCESS"
	JobConfigureWebsiteStatusCancelled   JobConfigureWebsiteStatus = "CANCELLED"
	JobConfigureWebsiteStatusTimeOut     JobConfigureWebsiteStatus = "TIMEOUT"
	JobConfigureWebsiteStatusFailed      JobConfigureWebsiteStatus = "FAILED"
)

type JobConfigureWebsiteResultByHost struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type HostUpdateResult struct {
	Status  JobConfigureWebsiteStatus `json:"status"`
	Error   *string                   `json:"error,omitempty"`
	Message string                    `json:"message"`

	HostID string     `json:"host_id"`
	JobID  string     `json:"job_id"`
	Time   *time.Time `json:"time"`
}

type ConfigureWebsiteRequest struct {
	// Website host name eg. example.com
	WebsiteHostAddress string `json:"website_host_address"`

	// artifactd repository ID
	RepositoryID string `json:"repository_id"`

	// build ID
	BuildID string `json:"build_id"`

	// The host which the configuration will be applied
	TargetHosts []string `json:"target_hosts"`

	Time time.Time `json:"time"`
}

type JobConfigureWebsite struct {
	Ns   string `json:"namespace"` // static: deployd
	Name string `json:"name"`      // static: configure-website

	Id string `json:"id"` // it's a version (auto increment)

	// main status / DAG
	Status string `json:"status"`

	Result map[string]*HostUpdateResult `json:"result"`

	Request ConfigureWebsiteRequest `json:"request"`

	Url         string     `json:"url"`
	PublishedAt time.Time  `json:"published_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

func (d *JobConfigureWebsite) CreatedTime() time.Time {
	return d.PublishedAt
}

func (d *JobConfigureWebsite) ID() string {
	return d.Id
}

func (d *JobConfigureWebsite) Namespace() string {
	return d.Ns
}

func (d *JobConfigureWebsite) RefIDs() []string {
	return []string{d.Name}
}

func (d *JobConfigureWebsite) URL() string {
	return d.Url
}

func (d *JobConfigureWebsite) Validate() error {
	// TODO: all need to be add validation eventually
	return nil
}

func (d *JobConfigureWebsite) WithCreatedTime(t time.Time) mycontent.Data {
	d.PublishedAt = t
	return d
}

func (d *JobConfigureWebsite) WithID(id string) mycontent.Data {
	d.Id = id
	return d
}

func (d *JobConfigureWebsite) WithNamespace(id string) mycontent.Data {
	d.Ns = id
	return d
}

func (d *JobConfigureWebsite) WithURL(url string) mycontent.Data {
	d.Url = url
	return d
}

func (d *JobConfigureWebsite) WithVersion(ver uint64) mycontent.Data {
	return d
}

func (d *JobConfigureWebsite) DGVersion() *uint64 {
	return nil
}
