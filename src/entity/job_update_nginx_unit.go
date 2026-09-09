package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

type (
	JobUpdateNginxUnitStatus string
)

const (
	// Overall status for the deployment job
	JobUpdateNginxUnitStatusQueued      JobUpdateNginxUnitStatus = "QUEUED"
	JobUpdateNginxUnitStatusConfiguring JobUpdateNginxUnitStatus = "CONFIGURING"
	JobUpdateNginxUnitStatusConfigured  JobUpdateNginxUnitStatus = "CONFIGURED"
	JobUpdateNginxUnitStatusDeploying   JobUpdateNginxUnitStatus = "DEPLOYING"
	JobUpdateNginxUnitStatusDeployed    JobUpdateNginxUnitStatus = "DEPLOYED"
	JobUpdateNginxUnitStatusSuccess     JobUpdateNginxUnitStatus = "SUCCESS"
	JobUpdateNginxUnitStatusCancelled   JobUpdateNginxUnitStatus = "CANCELLED"
	JobUpdateNginxUnitStatusTimeOut     JobUpdateNginxUnitStatus = "TIMEOUT"
	JobUpdateNginxUnitStatusFailed      JobUpdateNginxUnitStatus = "FAILED"
)

type JobUpdateNginxUnitResultByHost struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type JobUpdateNginxUnit struct {
	Ns string `json:"namespace"`
	Id string `json:"id"` // it's a version (auto increment)

	// main status / DAG
	Status string `json:"status"`

	Result map[string]JobUpdateNginxUnitStatus `json:"result"`

	// The request; when displaying can be omitted
	Request any `json:"request"`

	Url         string     `json:"url"`
	PublishedAt time.Time  `json:"published_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

func (d *JobUpdateNginxUnit) CreatedTime() time.Time {
	return d.PublishedAt
}

func (d *JobUpdateNginxUnit) ID() string {
	return d.Id
}

func (d *JobUpdateNginxUnit) Namespace() string {
	return d.Ns
}

func (d *JobUpdateNginxUnit) RefIDs() []string {
	return nil
}

func (d *JobUpdateNginxUnit) URL() string {
	return d.Url
}

func (d *JobUpdateNginxUnit) Validate() error {
	// TODO: all need to be add validation eventually
	return nil
}

func (d *JobUpdateNginxUnit) WithCreatedTime(t time.Time) mycontent.Data {
	d.PublishedAt = t
	return d
}

func (d *JobUpdateNginxUnit) WithID(id string) mycontent.Data {
	d.Id = id
	return d
}

func (d *JobUpdateNginxUnit) WithNamespace(id string) mycontent.Data {
	d.Ns = id
	return d
}

func (d *JobUpdateNginxUnit) WithURL(url string) mycontent.Data {
	d.Url = url
	return d
}

func (d *JobUpdateNginxUnit) WithVersion(ver uint64) mycontent.Data {
	return d
}

func (d *JobUpdateNginxUnit) DGVersion() *uint64 {
	return nil
}
