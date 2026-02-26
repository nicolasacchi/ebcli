package psu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	IPLookupURL     = "https://icanhazip.com"
	IPLookupTimeout = 5 * time.Second
)

// Provider generates PSU (Payment Service User) headers for API requests.
type Provider struct {
	version    string
	httpClient *http.Client

	mu       sync.Mutex
	cachedIP string
}

// NewProvider creates a new PSU header provider.
func NewProvider(version string) *Provider {
	return &Provider{
		version: version,
		httpClient: &http.Client{
			Timeout: IPLookupTimeout,
		},
	}
}

// Headers returns the PSU headers required by the given ASPSP.
// If requiredHeaders is empty, returns nil.
func (p *Provider) Headers(ctx context.Context, requiredHeaders []string) (http.Header, error) {
	if len(requiredHeaders) == 0 {
		return nil, nil
	}

	h := make(http.Header)
	for _, name := range requiredHeaders {
		switch strings.ToLower(name) {
		case "psu-ip-address":
			ip, err := p.detectIP(ctx)
			if err != nil {
				return nil, fmt.Errorf("detecting public IP for PSU header: %w", err)
			}
			h.Set("Psu-Ip-Address", ip)
		case "psu-user-agent":
			h.Set("Psu-User-Agent", "ebcli/"+p.version)
		case "psu-referer":
			h.Set("Psu-Referer", "ebcli")
		case "psu-accept":
			h.Set("Psu-Accept", "application/json")
		case "psu-accept-charset":
			h.Set("Psu-Accept-Charset", "utf-8")
		case "psu-accept-encoding":
			h.Set("Psu-Accept-Encoding", "gzip")
		case "psu-accept-language":
			h.Set("Psu-Accept-Language", "en")
		}
	}
	return h, nil
}

func (p *Provider) detectIP(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cachedIP != "" {
		return p.cachedIP, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, IPLookupURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching public IP: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading IP response: %w", err)
	}

	ip := strings.TrimSpace(string(body))
	if ip == "" {
		return "", fmt.Errorf("empty IP response from %s", IPLookupURL)
	}

	p.cachedIP = ip
	return ip, nil
}
