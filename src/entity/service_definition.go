package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

var _ mycontent.Data = &ServiceDefinition{}

type ServiceDefinition struct {
	Id   string `json:"id,omitempty"`
	Ns   string `json:"namespace" ch:"namespace"`
	Name string `json:"name" ch:"name"`

	PublishedAt time.Time `json:"published_at" ch:"published_at"`
	URLx        string    `json:"url"`
}

func (a *ServiceDefinition) CreatedTime() time.Time {
	return a.PublishedAt
}

func (a *ServiceDefinition) ID() string {
	return a.Id
}

func (a *ServiceDefinition) Namespace() string {
	return a.Ns
}

func (a *ServiceDefinition) RefIDs() []string {
	return []string{a.Id}
}

func (a *ServiceDefinition) URL() string {
	return a.URLx
}

func (a *ServiceDefinition) Validate() error {
	return nil
}

func (a *ServiceDefinition) WithCreatedTime(t time.Time) mycontent.Data {
	a.PublishedAt = t
	return a
}

func (a *ServiceDefinition) WithID(id string) mycontent.Data {
	a.Id = id
	return a
}

func (a *ServiceDefinition) WithNamespace(id string) mycontent.Data {
	a.Ns = id
	return a
}

func (a *ServiceDefinition) WithURL(url string) mycontent.Data {
	a.URLx = url
	return a
}
