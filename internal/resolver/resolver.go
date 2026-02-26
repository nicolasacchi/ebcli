package resolver

import (
	"fmt"
	"strings"

	"github.com/nicolasacchi/ebcli/internal/config"
)

// Result holds the resolved account and its parent connection info.
type Result struct {
	Account            config.Account
	Connection         config.Connection
	RequiredPSUHeaders []string
}

// Resolve finds an account across all connections in the config.
// The query can be an alias, UID, or IBAN (all case-insensitive).
func Resolve(cfg *config.Config, query string) (*Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty account identifier")
	}

	var matches []Result

	for _, conn := range cfg.Connections {
		for _, acct := range conn.Accounts {
			if matchesAccount(acct, query) {
				matches = append(matches, Result{
					Account:            acct,
					Connection:         conn,
					RequiredPSUHeaders: conn.RequiredPSUHeaders,
				})
			}
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no account found matching %q", query)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous identifier %q matches %d accounts; use a more specific identifier", query, len(matches))
	}
}

// ResolveAll returns all accounts from the config as resolved results.
func ResolveAll(cfg *config.Config) []Result {
	var results []Result
	for _, conn := range cfg.Connections {
		for _, acct := range conn.Accounts {
			results = append(results, Result{
				Account:            acct,
				Connection:         conn,
				RequiredPSUHeaders: conn.RequiredPSUHeaders,
			})
		}
	}
	return results
}

func matchesAccount(acct config.Account, query string) bool {
	q := strings.ToLower(query)

	// Alias match (case-insensitive)
	if strings.EqualFold(acct.Alias, query) {
		return true
	}

	// UID match (case-insensitive)
	if strings.EqualFold(acct.UID, query) {
		return true
	}

	// UID prefix match (minimum 4 chars)
	if len(q) >= 4 && strings.HasPrefix(strings.ToLower(acct.UID), q) {
		return true
	}

	// IBAN match (case-insensitive)
	if acct.IBAN != "" && strings.EqualFold(acct.IBAN, query) {
		return true
	}

	return false
}
