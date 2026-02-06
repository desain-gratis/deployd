package entity

import (
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

var (
	_ mycontent.Data = &KV{}

	_ mycontent.Data = &Secret{}
	_ mycontent.Data = &Env{}
)

// Represents key-value pair
type KV struct {
	Ns      string `json:"namespace"`
	Service string `json:"service"`

	Version string `json:"version"`

	Value map[string]string `json:"value"`

	PublishedAt time.Time `json:"published_at" ch:"published_at"`
	URLx        string    `json:"url"`
}

// Test composition, if it's awkward in the API, might need to create new struct
type Secret struct {
	KV
}

type Env struct {
	KV
}

func (a *KV) CreatedTime() time.Time {
	return a.PublishedAt
}

func (a *KV) ID() string {
	return a.Version
}

func (a *KV) Namespace() string {
	return a.Ns
}

func (a *KV) RefIDs() []string {
	return []string{a.Service}
}

func (a *KV) URL() string {
	return a.URLx
}

func (a *KV) Validate() error {
	// TODO: validate raw json
	// a.Replica[x].Config
	return nil
}

func (a *KV) WithCreatedTime(t time.Time) mycontent.Data {
	a.PublishedAt = t
	return a
}

func (a *KV) WithID(id string) mycontent.Data {
	a.Version = id
	return a
}

func (a *KV) WithNamespace(ns string) mycontent.Data {
	a.Ns = ns
	return a
}

func (a *KV) WithURL(url string) mycontent.Data {
	a.URLx = url
	return a
}
