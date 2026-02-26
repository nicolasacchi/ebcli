package resolver

import (
	"testing"

	"github.com/nicolasacchi/ebcli/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		Connections: []config.Connection{
			{
				Name:         "nordea",
				ASPSPName:    "Nordea",
				ASPSPCountry: "FI",
				Accounts: []config.Account{
					{
						UID:      "07cc67f4-1234-5678-9abc-def012345678",
						IBAN:     "FI1234567890123456",
						Alias:    "nordea-eur",
						Currency: "EUR",
					},
					{
						UID:      "08dd78e5-2345-6789-abcd-ef0123456789",
						IBAN:     "FI9876543210987654",
						Alias:    "nordea-eur-2",
						Currency: "EUR",
					},
				},
			},
			{
				Name:         "revolut",
				ASPSPName:    "Revolut",
				ASPSPCountry: "LT",
				Accounts: []config.Account{
					{
						UID:      "a1b2c3d4-e5f6-7890-abcd-ef0123456789",
						IBAN:     "LT891234567890",
						Alias:    "revolut-eur",
						Currency: "EUR",
					},
				},
			},
		},
	}
}

func TestResolve_ByAlias(t *testing.T) {
	result, err := Resolve(testConfig(), "nordea-eur")
	if err != nil {
		t.Fatalf("Resolve by alias: %v", err)
	}
	if result.Account.IBAN != "FI1234567890123456" {
		t.Errorf("IBAN = %q, want FI1234567890123456", result.Account.IBAN)
	}
}

func TestResolve_ByAliasCaseInsensitive(t *testing.T) {
	result, err := Resolve(testConfig(), "NORDEA-EUR")
	if err != nil {
		t.Fatalf("Resolve by alias (case-insensitive): %v", err)
	}
	if result.Account.Alias != "nordea-eur" {
		t.Errorf("Alias = %q, want nordea-eur", result.Account.Alias)
	}
}

func TestResolve_ByUID(t *testing.T) {
	result, err := Resolve(testConfig(), "07cc67f4-1234-5678-9abc-def012345678")
	if err != nil {
		t.Fatalf("Resolve by UID: %v", err)
	}
	if result.Account.Alias != "nordea-eur" {
		t.Errorf("Alias = %q, want nordea-eur", result.Account.Alias)
	}
}

func TestResolve_ByUIDPrefix(t *testing.T) {
	result, err := Resolve(testConfig(), "07cc67f4")
	if err != nil {
		t.Fatalf("Resolve by UID prefix: %v", err)
	}
	if result.Account.Alias != "nordea-eur" {
		t.Errorf("Alias = %q, want nordea-eur", result.Account.Alias)
	}
}

func TestResolve_ByIBAN(t *testing.T) {
	result, err := Resolve(testConfig(), "LT891234567890")
	if err != nil {
		t.Fatalf("Resolve by IBAN: %v", err)
	}
	if result.Account.Alias != "revolut-eur" {
		t.Errorf("Alias = %q, want revolut-eur", result.Account.Alias)
	}
}

func TestResolve_NotFound(t *testing.T) {
	_, err := Resolve(testConfig(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent account")
	}
}

func TestResolve_Empty(t *testing.T) {
	_, err := Resolve(testConfig(), "")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestResolveAll(t *testing.T) {
	results := ResolveAll(testConfig())
	if len(results) != 3 {
		t.Fatalf("ResolveAll = %d accounts, want 3", len(results))
	}
}

func TestResolveAll_Empty(t *testing.T) {
	results := ResolveAll(&config.Config{})
	if len(results) != 0 {
		t.Fatalf("ResolveAll on empty config = %d, want 0", len(results))
	}
}
