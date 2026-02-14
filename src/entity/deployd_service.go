package entity

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

const (
	maxIdLength = 64
)

var (
	_ mycontent.Data = &ServiceDefinition{}

	alphanumeric = regexp.MustCompile("^[a-zA-Z0-9]+$")
)

type ServiceDefinition struct {
	Ns string `json:"namespace"`

	Id string `json:"id"`

	Name        string `json:"name"`
	Description string `json:"description"`

	Repository     ArtifactdRepository `json:"repository"`
	ExecutablePath string              `json:"executable_path"` // todo add validation

	BoundAddresses []BoundAddress `json:"bound_addresses"`

	PublishedAt time.Time `json:"published_at"`
	URLx        string    `json:"url"`
}

type BoundAddress struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type ArtifactdRepository struct {
	URL string `json:"url"`       // in case external
	Ns  string `json:"namespace"` // in case external
	ID  string `json:"id"`
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
	return nil
}

func (a *ServiceDefinition) URL() string {
	return a.URLx
}

func (a *ServiceDefinition) Validate() error {
	var validationErrs error

	if strings.TrimSpace(a.Id) != a.Id {
		errors.Join(fmt.Errorf("%v: id cannot contain empty space (found: '%v')", mycontent.ErrValidation, len(a.Id)))
	}

	if len(strings.Fields(a.Id)) > 0 {
		errors.Join(fmt.Errorf("%v: id cannot be separated by empty space (found: '%v')", mycontent.ErrValidation, a.Id))
	}

	if len(a.Id) > maxIdLength {
		errors.Join(fmt.Errorf("%v: id length cannot be greater than %v (found: %v)", mycontent.ErrValidation, maxIdLength, len(a.Id)))
	}

	if !alphanumeric.MatchString(a.Id) {
		errors.Join(fmt.Errorf("%v: id must only be an alphanumeric string (found: '%v')", mycontent.ErrValidation, a.Id))
	}

	return validationErrs
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
