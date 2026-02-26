package config

import "time"

// Config represents the top-level configuration stored at ~/.config/ebcli/config.json.
type Config struct {
	AppID          string       `json:"app_id"`
	PrivateKeyPath string       `json:"private_key_path"`
	Environment    string       `json:"environment"` // "PRODUCTION" or "SANDBOX"
	Connections    []Connection `json:"connections"`
}

// Connection represents an authorized bank session.
type Connection struct {
	Name                      string    `json:"name"`
	ASPSPCountry              string    `json:"aspsp_country"`
	ASPSPName                 string    `json:"aspsp_name"`
	SessionID                 string    `json:"session_id"`
	Accounts                  []Account `json:"accounts"`
	ConnectedAt               time.Time `json:"connected_at"`
	ValidUntil                time.Time `json:"valid_until"`
	MaxConsentValiditySeconds int       `json:"max_consent_validity_seconds"`
	RequiredPSUHeaders        []string  `json:"required_psu_headers,omitempty"`
}

// Account represents a single bank account within a connection.
type Account struct {
	UID                string `json:"uid"`
	IBAN               string `json:"iban,omitempty"`
	Alias              string `json:"alias"`
	Currency           string `json:"currency"`
	IdentificationHash string `json:"identification_hash"`
	CashAccountType    string `json:"cash_account_type"`
}
