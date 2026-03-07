package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type CreateTunnelRequest struct {
	Name      string `json:"name"`
	ConfigSrc string `json:"config_src"`
}

func CreateTunnel(ctx context.Context, accountID, apiToken, name string) (*CreateTunnelResponse, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel", accountID)

	reqBody := CreateTunnelRequest{
		Name:      name,
		ConfigSrc: "cloudflare",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var cfResp CreateTunnelResponse
	if err := json.Unmarshal(raw, &cfResp); err != nil {
		return nil, err
	}

	if !cfResp.Success {
		return &cfResp, fmt.Errorf("cloudflare API error: %s", raw)
	}

	return &cfResp, nil
}

// Or, PublishApp
func UpdateTunnelConfig(
	ctx context.Context,
	accountID string,
	tunnelID string,
	apiToken string,
	config TunnelConfig,
) (*TunnelConfigResponse, error) {

	url := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations",
		accountID,
		tunnelID,
	)

	reqBody := TunnelConfigRequest{
		Config: config,
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		url,
		bytes.NewReader(b),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out TunnelConfigResponse
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return nil, err
	}

	if !out.Success {
		return &out, fmt.Errorf("cloudflare API returned error")
	}

	return &out, nil
}

func CreateDNSRecord(
	ctx context.Context,
	zoneID string,
	apiToken string,
	record CreateDNSRecordRequest,
) (*DNSRecordResponse, error) {

	url := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/zones/%s/dns_records",
		zoneID,
	)

	body, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out DNSRecordResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	if !out.Success {
		return &out, fmt.Errorf("cloudflare DNS create failed")
	}

	return &out, nil
}

func GetTunnel(
	ctx context.Context,
	accountID string,
	tunnelID string,
	apiToken string,
) (*GetTunnelResponse, error) {

	url := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s",
		accountID,
		tunnelID,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out GetTunnelResponse

	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return nil, err
	}

	if !out.Success {
		return &out, fmt.Errorf("cloudflare returned error")
	}

	return &out, nil
}
