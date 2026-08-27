package entity

import (
	"strconv"
	"time"

	"github.com/desain-gratis/common/delivery/mycontent-api/mycontent"
)

var _ mycontent.Data = &Routing{}

// Routing represents cloudflare config
type Routing struct {
	Ns      string `json:"namespace"`
	Service string `json:"service"`
	Version string `json:"version"`

	CloudflareConfig *CloudflareConfig `json:"cloudflare_config,omitempty"`

	PublishedAt time.Time `json:"published_at"`
	URLx        string    `json:"url"`
}

type CloudflareConfig struct {
	TunnelToken string `json:"tunnel_token"`
}

func (c *Routing) CreatedTime() time.Time {
	return c.PublishedAt
}

func (c *Routing) ID() string {
	return c.Service
}

func (c *Routing) Namespace() string {
	return c.Ns
}

func (c *Routing) RefIDs() []string {
	return nil
}

func (c *Routing) URL() string {
	return c.URLx
}

func (c *Routing) Validate() error {
	return nil
}

func (c *Routing) WithCreatedTime(t time.Time) mycontent.Data {
	c.PublishedAt = t
	return c
}

func (c *Routing) WithID(id string) mycontent.Data {
	c.Service = id
	return c
}

func (c *Routing) WithNamespace(ns string) mycontent.Data {
	c.Ns = ns
	return c
}

func (c *Routing) WithURL(url string) mycontent.Data {
	c.URLx = url
	return c
}

func (a *Routing) WithVersion(ver uint64) mycontent.Data {
	a.Version = strconv.FormatUint(ver, 10)
	return a
}
