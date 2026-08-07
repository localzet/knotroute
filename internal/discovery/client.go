package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/networkid"
)

type Client struct{ HTTP *http.Client }

func ValidateBeaconURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("beacon URL is empty")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("beacon URL must be an HTTP(S) URL, for example https://beacon.example.net")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("beacon URL must not contain credentials, query parameters, or a fragment")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("beacon URL must point to the Beacon API root, not %q", u.Path)
	}
	if u.Port() == "7447" {
		return "", fmt.Errorf("port 7447 is the KnotRoute relay transport, not the Beacon HTTP API; use the HTTPS Beacon URL or its HTTP port (default 8080)")
	}
	u.Path = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func (c *Client) Exchange(ctx context.Context, rawURL string, id *identity.Identity, network networkid.ID, endpoints []string) ([]Candidate, error) {
	baseURL, err := ValidateBeaconURL(rawURL)
	if err != nil {
		return nil, err
	}
	a := SignAnnouncement(id, network, endpoints, time.Now())
	raw, _ := json.Marshal(a)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/peers", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 8 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("beacon %s: %w", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail != "" {
			return nil, fmt.Errorf("beacon %s returned %s: %s", baseURL, resp.Status, detail)
		}
		return nil, fmt.Errorf("beacon %s returned %s", baseURL, resp.Status)
	}
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("beacon %s returned invalid JSON: %w", baseURL, err)
	}
	return out.Peers, nil
}
