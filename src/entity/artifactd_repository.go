package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

var _ mycontent.Data = &Repository{}

type Repository struct {
	Id          string    `json:"id"`
	Ns          string    `json:"namespace"`
	Name        string    `json:"name"`
	Source      string    `json:"source"`
	PublishedAt time.Time `json:"published_at"`
	URLx        string    `json:"url"`

	// TODO: generate API KEY / API SECRET to validate archive upload in /secretd
}

func (r *Repository) CreatedTime() time.Time {
	return r.PublishedAt
}

func (r *Repository) ID() string {
	return r.Id
}

func (r *Repository) Namespace() string {
	return r.Ns
}

func (r *Repository) RefIDs() []string {
	return nil
}

func (r *Repository) URL() string {
	return r.URLx
}

func (r *Repository) Validate() error {
	return nil
}

func (r *Repository) WithCreatedTime(t time.Time) mycontent.Data {
	r.PublishedAt = t
	return r
}

func (r *Repository) WithID(id string) mycontent.Data {
	r.Id = id
	return r
}

func (r *Repository) WithNamespace(id string) mycontent.Data {
	r.Ns = id
	return r
}

func (r *Repository) WithURL(url string) mycontent.Data {
	r.URLx = url
	return r
}
