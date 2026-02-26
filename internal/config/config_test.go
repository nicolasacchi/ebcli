package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &Config{
		AppID:          "test-app-id",
		PrivateKeyPath: "/path/to/key.pem",
		Environment:    "SANDBOX",
		Connections: []Connection{
			{
				Name:         "test-conn",
				ASPSPCountry: "FI",
				ASPSPName:    "Nordea",
				SessionID:    "sess-123",
				Accounts: []Account{
					{
						UID:                "acct-uid-1",
						IBAN:               "FI1234567890",
						Alias:              "nordea-eur",
						Currency:           "EUR",
						IdentificationHash: "hash123",
						CashAccountType:    "CACC",
					},
				},
				ConnectedAt: time.Now().Truncate(time.Second),
				ValidUntil:  time.Now().Add(90 * 24 * time.Hour).Truncate(time.Second),
			},
		},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != FilePermissions {
		t.Errorf("permissions = %o, want %o", perm, FilePermissions)
	}

	// Load and verify
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.AppID != cfg.AppID {
		t.Errorf("AppID = %q, want %q", loaded.AppID, cfg.AppID)
	}
	if loaded.Environment != cfg.Environment {
		t.Errorf("Environment = %q, want %q", loaded.Environment, cfg.Environment)
	}
	if len(loaded.Connections) != 1 {
		t.Fatalf("Connections = %d, want 1", len(loaded.Connections))
	}
	if loaded.Connections[0].Name != "test-conn" {
		t.Errorf("Connection.Name = %q, want %q", loaded.Connections[0].Name, "test-conn")
	}
	if len(loaded.Connections[0].Accounts) != 1 {
		t.Fatalf("Accounts = %d, want 1", len(loaded.Connections[0].Accounts))
	}
	if loaded.Connections[0].Accounts[0].IBAN != "FI1234567890" {
		t.Errorf("Account.IBAN = %q, want %q", loaded.Connections[0].Accounts[0].IBAN, "FI1234567890")
	}
}

func TestLoad_NotFound(t *testing.T) {
	cfg, err := Load("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("Load nonexistent: %v", err)
	}
	if cfg.AppID != "" {
		t.Errorf("AppID should be empty for nonexistent config")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte("{invalid"), 0600)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("EBCLI_APP_ID", "env-app-id")
	t.Setenv("EBCLI_PRIVATE_KEY", "/env/key.pem")

	cfg, err := Load("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.AppID != "env-app-id" {
		t.Errorf("AppID = %q, want %q", cfg.AppID, "env-app-id")
	}
	if cfg.PrivateKeyPath != "/env/key.pem" {
		t.Errorf("PrivateKeyPath = %q, want %q", cfg.PrivateKeyPath, "/env/key.pem")
	}
}

func TestAddConnection(t *testing.T) {
	cfg := &Config{}

	conn := Connection{Name: "test"}
	if err := cfg.AddConnection(conn); err != nil {
		t.Fatalf("AddConnection: %v", err)
	}
	if len(cfg.Connections) != 1 {
		t.Fatalf("Connections = %d, want 1", len(cfg.Connections))
	}

	// Duplicate should fail
	if err := cfg.AddConnection(conn); err == nil {
		t.Fatal("expected error for duplicate connection")
	}
}

func TestRemoveConnection(t *testing.T) {
	cfg := &Config{
		Connections: []Connection{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
		},
	}

	if err := cfg.RemoveConnection("b"); err != nil {
		t.Fatalf("RemoveConnection: %v", err)
	}
	if len(cfg.Connections) != 2 {
		t.Fatalf("Connections = %d, want 2", len(cfg.Connections))
	}
	if cfg.Connections[0].Name != "a" || cfg.Connections[1].Name != "c" {
		t.Errorf("wrong connections after remove: %v", cfg.Connections)
	}

	// Not found
	if err := cfg.RemoveConnection("x"); err == nil {
		t.Fatal("expected error for nonexistent connection")
	}
}

func TestFindConnection(t *testing.T) {
	cfg := &Config{
		Connections: []Connection{
			{Name: "MyBank"},
		},
	}

	// Case-insensitive
	conn, err := cfg.FindConnection("mybank")
	if err != nil {
		t.Fatalf("FindConnection: %v", err)
	}
	if conn.Name != "MyBank" {
		t.Errorf("Name = %q, want %q", conn.Name, "MyBank")
	}

	// Not found
	_, err = cfg.FindConnection("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent connection")
	}
}

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"~/test", filepath.Join(home, "test")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		got, err := ExpandTilde(tt.input)
		if err != nil {
			t.Errorf("ExpandTilde(%q): %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ExpandTilde(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
