package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

type Website struct {
	Ns          string `json:"namespace"`
	HostAddress string `json:"host_address"`

	Name        string `json:"name"`
	Description string `json:"description"`

	ArtifactID string `json:"artifact_id"` // artifact used

	Custom404Page string `json:"custom_404_page,omitempty"`

	ListenAddress string `json:"listen_address,omitempty"`

	Url         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

func (d *Website) CreatedTime() time.Time {
	return d.PublishedAt
}

func (d *Website) ID() string {
	return d.HostAddress
}

func (d *Website) Namespace() string {
	return d.Ns
}

func (d *Website) RefIDs() []string {
	return nil
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
	d.HostAddress = id
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
