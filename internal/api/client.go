package api

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicolasacchi/ebcli/internal/auth"
	"github.com/nicolasacchi/ebcli/internal/psu"
	"github.com/nicolasacchi/ebcli/internal/ratelimit"
)

const (
	BaseURL        = "https://api.enablebanking.com"
	requestTimeout = 30 * time.Second
)

// Client is the Enable Banking API client.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	appID       string
	privateKey  *rsa.PrivateKey
	psuProvider *psu.Provider
	rateLimiter *ratelimit.Tracker
	version     string
}

// ClientOption is a functional option for configuring the Client.
type ClientOption func(*Client)

// NewClient creates a new API client.
func NewClient(appID string, privateKey *rsa.PrivateKey, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: requestTimeout},
		baseURL:    BaseURL,
		appID:      appID,
		privateKey: privateKey,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = strings.TrimRight(url, "/") }
}

func WithPSUProvider(p *psu.Provider) ClientOption {
	return func(c *Client) { c.psuProvider = p }
}

func WithRateLimiter(r *ratelimit.Tracker) ClientOption {
	return func(c *Client) { c.rateLimiter = r }
}

func WithVersion(v string) ClientOption {
	return func(c *Client) { c.version = v }
}

// doRequest performs an authenticated HTTP request with retry logic.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}, requiredPSUHeaders []string) ([]byte, error) {
	return c.doRequestWithRetry(ctx, method, path, body, result, requiredPSUHeaders, true)
}

func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body interface{}, result interface{}, requiredPSUHeaders []string, canRetry bool) ([]byte, error) {
	// Generate JWT
	token, err := auth.GenerateJWT(c.privateKey, c.appID, auth.DefaultTTL)
	if err != nil {
		return nil, fmt.Errorf("generating JWT: %w", err)
	}

	// Marshal body
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	// Create request
	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Inject PSU headers if needed
	if len(requiredPSUHeaders) > 0 && c.psuProvider != nil {
		psuHeaders, err := c.psuProvider.Headers(ctx, requiredPSUHeaders)
		if err != nil {
			return nil, fmt.Errorf("generating PSU headers: %w", err)
		}
		for k, vals := range psuHeaders {
			for _, v := range vals {
				req.Header.Set(k, v)
			}
		}
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if canRetry {
			time.Sleep(500 * time.Millisecond)
			return c.doRequestWithRetry(ctx, method, path, body, result, requiredPSUHeaders, false)
		}
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// Track rate limits
	if c.rateLimiter != nil {
		accountUID := extractAccountUID(path)
		endpoint := extractEndpoint(path)
		c.rateLimiter.Update(accountUID, endpoint, resp)
	}

	// Handle status codes
	if resp.StatusCode == 429 && canRetry {
		if c.rateLimiter != nil {
			accountUID := extractAccountUID(path)
			endpoint := extractEndpoint(path)
			if retryAt, ok := ratelimit.ParseRetryAfter(resp); ok {
				c.rateLimiter.RecordRetryAfter(accountUID, endpoint, retryAt)
				waitDur := time.Until(retryAt)
				if waitDur > 0 && waitDur < 5*time.Minute {
					time.Sleep(waitDur)
					return c.doRequestWithRetry(ctx, method, path, body, result, requiredPSUHeaders, false)
				}
			}
		}
		// Default backoff for 429
		time.Sleep(2 * time.Second)
		return c.doRequestWithRetry(ctx, method, path, body, result, requiredPSUHeaders, false)
	}

	if resp.StatusCode >= 500 && canRetry {
		time.Sleep(500 * time.Millisecond)
		return c.doRequestWithRetry(ctx, method, path, body, result, requiredPSUHeaders, false)
	}

	if resp.StatusCode >= 400 {
		// Check for session expired
		if (resp.StatusCode == 401 || resp.StatusCode == 403) && strings.Contains(path, "/accounts/") {
			return nil, &SessionExpiredError{
				SessionID: extractAccountUID(path),
				Wrapped:   parseAPIError(resp.StatusCode, respBody),
			}
		}
		return nil, parseAPIError(resp.StatusCode, respBody)
	}

	// Success — unmarshal if result provided
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return respBody, fmt.Errorf("parsing response: %w", err)
		}
	}

	return respBody, nil
}

// --- Public API methods ---

func (c *Client) ListASPSPs(ctx context.Context, country, psuType string) ([]ASPSPData, error) {
	path := "/aspsps?country=" + url.QueryEscape(country) + "&service=AIS"
	if psuType != "" {
		path += "&psu_type=" + url.QueryEscape(psuType)
	}
	var result []ASPSPData
	_, err := c.doRequest(ctx, http.MethodGet, path, nil, &result, nil)
	return result, err
}

func (c *Client) Authorize(ctx context.Context, req *AuthRequest) (*AuthResponse, error) {
	var result AuthResponse
	_, err := c.doRequest(ctx, http.MethodPost, "/auth", req, &result, nil)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateSession(ctx context.Context, code string) (*SessionResponse, error) {
	var result SessionResponse
	_, err := c.doRequest(ctx, http.MethodPost, "/sessions", &SessionRequest{Code: code}, &result, nil)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetSession(ctx context.Context, sessionID string) (*SessionStatus, error) {
	var result SessionStatus
	_, err := c.doRequest(ctx, http.MethodGet, "/sessions/"+url.PathEscape(sessionID), nil, &result, nil)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteSession(ctx context.Context, sessionID string, psuHeaders []string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/sessions/"+url.PathEscape(sessionID), nil, nil, psuHeaders)
	return err
}

func (c *Client) GetAccountDetails(ctx context.Context, accountUID string, psuHeaders []string) (*AccountDetails, error) {
	var result AccountDetails
	_, err := c.doRequest(ctx, http.MethodGet, "/accounts/"+url.PathEscape(accountUID)+"/details", nil, &result, psuHeaders)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetBalances(ctx context.Context, accountUID string, psuHeaders []string) (*BalancesResponse, error) {
	var result BalancesResponse
	_, err := c.doRequest(ctx, http.MethodGet, "/accounts/"+url.PathEscape(accountUID)+"/balances", nil, &result, psuHeaders)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetTransactions(ctx context.Context, accountUID string, params TransactionParams, psuHeaders []string) (*TransactionsResponse, error) {
	path := "/accounts/" + url.PathEscape(accountUID) + "/transactions"
	q := url.Values{}
	if params.DateFrom != "" {
		q.Set("date_from", params.DateFrom)
	}
	if params.DateTo != "" {
		q.Set("date_to", params.DateTo)
	}
	if params.ContinuationKey != "" {
		q.Set("continuation_key", params.ContinuationKey)
	}
	if params.TransactionStatus != "" {
		q.Set("transaction_status", params.TransactionStatus)
	}
	if qs := q.Encode(); qs != "" {
		path += "?" + qs
	}

	var result TransactionsResponse
	_, err := c.doRequest(ctx, http.MethodGet, path, nil, &result, psuHeaders)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetApplication(ctx context.Context) (*ApplicationResponse, error) {
	var result ApplicationResponse
	_, err := c.doRequest(ctx, http.MethodGet, "/application", nil, &result, nil)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// DoRaw performs a request and returns the raw response body.
func (c *Client) DoRaw(ctx context.Context, method, path string, body interface{}, psuHeaders []string) ([]byte, error) {
	return c.doRequest(ctx, method, path, body, nil, psuHeaders)
}

// --- Helpers ---

func parseAPIError(statusCode int, body []byte) *APIError {
	apiErr := &APIError{StatusCode: statusCode}
	// Try to parse as Enable Banking error format
	var parsed struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		Detail           string `json:"detail"`
		Message          string `json:"message"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		apiErr.ErrorCode = parsed.Error
		apiErr.ErrorDescription = parsed.ErrorDescription
		if apiErr.ErrorCode == "" {
			apiErr.ErrorCode = parsed.Detail
		}
		if apiErr.ErrorCode == "" {
			apiErr.ErrorCode = parsed.Message
		}
	}
	if apiErr.ErrorCode == "" {
		apiErr.ErrorCode = http.StatusText(statusCode)
	}
	return apiErr
}

// extractAccountUID extracts the account UID from API paths like /accounts/{uid}/balances.
func extractAccountUID(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "accounts" && i+1 < len(parts) {
			uid := parts[i+1]
			// Strip query string
			if idx := strings.Index(uid, "?"); idx != -1 {
				uid = uid[:idx]
			}
			return uid
		}
	}
	return ""
}

// extractEndpoint extracts the endpoint name from API paths.
func extractEndpoint(path string) string {
	// Strip query string
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}
