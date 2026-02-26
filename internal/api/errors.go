package api

import "fmt"

// APIError represents an error response from the Enable Banking API.
type APIError struct {
	StatusCode       int    `json:"status_code"`
	ErrorCode        string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func (e *APIError) Error() string {
	if e.ErrorDescription != "" {
		return fmt.Sprintf("API error %d: %s — %s", e.StatusCode, e.ErrorCode, e.ErrorDescription)
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.ErrorCode)
}

// ExitCode maps HTTP status codes to CLI exit codes.
func (e *APIError) ExitCode() int {
	switch {
	case e.StatusCode == 401 || e.StatusCode == 403:
		return 3 // auth error
	case e.StatusCode >= 400 && e.StatusCode < 500:
		return 1 // user error
	default:
		return 2 // API/server error
	}
}

// IsRetryable returns true for 5xx and 429 status codes.
func (e *APIError) IsRetryable() bool {
	return e.StatusCode == 429 || e.StatusCode >= 500
}

// SessionExpiredError indicates the session is no longer valid.
type SessionExpiredError struct {
	SessionID string
	Wrapped   error
}

func (e *SessionExpiredError) Error() string {
	return fmt.Sprintf("session %s expired or revoked; run 'ebcli connect' to re-authorize", e.SessionID)
}

func (e *SessionExpiredError) Unwrap() error {
	return e.Wrapped
}
