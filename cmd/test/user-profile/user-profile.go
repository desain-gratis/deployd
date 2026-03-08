package main

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

type UserProfile struct {
	Ns        string    `json:"namespace"`
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	Domicile  string    `json:"domicile"`
	UpdatedAt time.Time `json:"updated_at"`
	Url       string    `json:"url"`
}

func (a *UserProfile) CreatedTime() time.Time {
	return a.UpdatedAt
}

func (a *UserProfile) ID() string {
	return a.Id
}

func (a *UserProfile) Namespace() string {
	return a.Ns
}

func (a *UserProfile) RefIDs() []string {
	return nil
}

func (a *UserProfile) URL() string {
	return a.Url
}

func (a *UserProfile) Validate() error {
	// TODO: validate raw json
	// a.Replica[x].Config
	return nil
}

func (a *UserProfile) WithCreatedTime(t time.Time) mycontent.Data {
	a.UpdatedAt = t
	return a
}

func (a *UserProfile) WithID(id string) mycontent.Data {
	a.Id = id
	return a
}

func (a *UserProfile) WithNamespace(ns string) mycontent.Data {
	a.Ns = ns
	return a
}

func (a *UserProfile) WithURL(url string) mycontent.Data {
	a.Url = url
	return a
}
