package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

type Website struct {
	Ns       string `json:"namespace"`
	HostName string `json:"host_name"`

	Name        string `json:"name"`
	Description string `json:"description"`

	ArtifactID string `json:"artifact_id"` // artifact used

	Url         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

func (d *Website) CreatedTime() time.Time {
	return d.PublishedAt
}

func (d *Website) ID() string {
	return d.HostName
}

func (d *Website) Namespace() string {
	return d.Ns
}

func (d *Website) RefIDs() []string {
	return []string{d.Name}
}

func (d *Website) URL() string {
	return d.Url
}

func (d *Website) Validate() error {
	// TODO: all need to be add validation eventually
	return nil
}

func (d *Website) WithCreatedTime(t time.Time) mycontent.Data {
	d.PublishedAt = t
	return d
}

func (d *Website) WithID(id string) mycontent.Data {
	d.HostName = id
	return d
}

func (d *Website) WithNamespace(id string) mycontent.Data {
	d.Ns = id
	return d
}

func (d *Website) WithURL(url string) mycontent.Data {
	d.Url = url
	return d
}

func (d *Website) WithVersion(ver uint64) mycontent.Data {
	return d
}

func (d *Website) DGVersion() *uint64 {
	return nil
}
