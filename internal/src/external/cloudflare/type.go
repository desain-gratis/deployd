package cloudflare

import "time"

type CreateTunnelResponse struct {
	Success  bool   `json:"success"`
	Errors   []any  `json:"errors"`
	Messages []any  `json:"messages"`
	Result   Tunnel `json:"result"`
}

type Tunnel struct {
	ID              string          `json:"id"`
	AccountTag      string          `json:"account_tag"`
	CreatedAt       time.Time       `json:"created_at"`
	DeletedAt       *time.Time      `json:"deleted_at"`
	Name            string          `json:"name"`
	Connections     []Connection    `json:"connections"`
	ConnsActiveAt   *time.Time      `json:"conns_active_at"`
	ConnsInactiveAt *time.Time      `json:"conns_inactive_at"`
	TunType         string          `json:"tun_type"`
	Metadata        map[string]any  `json:"metadata"`
	Status          string          `json:"status"`
	RemoteConfig    bool            `json:"remote_config"`
	CredentialsFile CredentialsFile `json:"credentials_file"`
	Token           string          `json:"token"`
}

type Connection struct {
	// currently empty in response example
}

type CredentialsFile struct {
	AccountTag   string `json:"AccountTag"`
	TunnelID     string `json:"TunnelID"`
	TunnelName   string `json:"TunnelName"`
	TunnelSecret string `json:"TunnelSecret"`
}

type TunnelConfigRequest struct {
	Config TunnelConfig `json:"config"`
}

type TunnelConfig struct {
	Ingress []IngressRule `json:"ingress"`
}

type IngressRule struct {
	Hostname      string         `json:"hostname,omitempty"`
	Service       string         `json:"service"`
	OriginRequest map[string]any `json:"originRequest,omitempty"`
}

type TunnelConfigResponse struct {
	Success  bool  `json:"success"`
	Errors   []any `json:"errors"`
	Messages []any `json:"messages"`
	Result   any   `json:"result"`
}

type CreateDNSRecordRequest struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

type DNSRecordResponse struct {
	Success  bool      `json:"success"`
	Errors   []any     `json:"errors"`
	Messages []any     `json:"messages"`
	Result   DNSRecord `json:"result"`
}

type DNSRecord struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Proxied   bool      `json:"proxied"`
	TTL       int       `json:"ttl"`
	CreatedAt time.Time `json:"created_on"`
	UpdatedAt time.Time `json:"modified_on"`
}

type GetTunnelResponse struct {
	Success  bool   `json:"success"`
	Errors   []any  `json:"errors"`
	Messages []any  `json:"messages"`
	Result   Tunnel `json:"result"`
}
