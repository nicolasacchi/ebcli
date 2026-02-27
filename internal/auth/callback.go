package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	DefaultCallbackPort = 18271
	CallbackTimeout     = 5 * time.Minute
	CallbackPath        = "/callback"
)

// CallbackResult holds the data received from the OAuth redirect.
type CallbackResult struct {
	Code             string
	State            string
	Error            string
	ErrorDescription string
}

// ListenForCallback starts a local HTTP server on the given port,
// waits for a single OAuth callback, validates the state parameter,
// and returns the authorization code.
func ListenForCallback(ctx context.Context, port int, expectedState string) (*CallbackResult, error) {
	resultCh := make(chan *CallbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		result := &CallbackResult{
			Code:             r.URL.Query().Get("code"),
			State:            r.URL.Query().Get("state"),
			Error:            r.URL.Query().Get("error"),
			ErrorDescription: r.URL.Query().Get("error_description"),
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
<h2>Authorization complete</h2>
<p>You can close this tab and return to the terminal.</p>
<script>window.close()</script>
</body></html>`)

		select {
		case resultCh <- result:
		default:
		}
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Check port availability
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, fmt.Errorf("port %d unavailable: %w", port, err)
	}

	go func() {
		server.Serve(ln)
	}()

	ctx, cancel := context.WithTimeout(ctx, CallbackTimeout)
	defer cancel()

	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		server.Shutdown(shutdownCtx)
	}()

	select {
	case result := <-resultCh:
		// Validate state parameter (CSRF protection)
		if result.State != expectedState {
			return nil, fmt.Errorf("state mismatch: expected %q, got %q (possible CSRF attack)", expectedState, result.State)
		}

		// Check for error from bank
		if result.Error != "" {
			return result, fmt.Errorf("authorization failed: %s — %s", result.Error, result.ErrorDescription)
		}

		if result.Code == "" {
			return nil, fmt.Errorf("no authorization code received in callback")
		}

		return result, nil

	case <-ctx.Done():
		return nil, fmt.Errorf("timed out waiting for bank authorization (waited %v)", CallbackTimeout)
	}
}

// CallbackURL returns the full redirect URL for a given port.
// If override is non-empty, it is returned as-is (for public callback URLs).
func CallbackURL(port int, override string) string {
	if override != "" {
		return override
	}
	return fmt.Sprintf("http://localhost:%d%s", port, CallbackPath)
}
