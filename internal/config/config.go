package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultConfigDir  = ".config/ebcli"
	DefaultConfigFile = "config.json"
	FilePermissions   = os.FileMode(0600)
	DirPermissions    = os.FileMode(0700)
)

// Paths resolves the config directory and file path.
// Checks EBCLI_CONFIG env var first, then falls back to ~/.config/ebcli/config.json.
func Paths(override string) (dir string, filePath string, err error) {
	if override != "" {
		expanded, err := ExpandTilde(override)
		if err != nil {
			return "", "", err
		}
		return filepath.Dir(expanded), expanded, nil
	}

	if envPath := os.Getenv("EBCLI_CONFIG"); envPath != "" {
		expanded, err := ExpandTilde(envPath)
		if err != nil {
			return "", "", err
		}
		return filepath.Dir(expanded), expanded, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	dir = filepath.Join(home, DefaultConfigDir)
	filePath = filepath.Join(dir, DefaultConfigFile)
	return dir, filePath, nil
}

// Load reads the config file from disk. Returns empty Config if file doesn't exist.
// Env var overrides are applied after loading.
func Load(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

// Save writes the config to disk with an exclusive file lock.
// Creates the directory with 0700 and file with 0600 permissions.
// Uses atomic write: write to temp file in same dir, then rename.
func Save(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Acquire exclusive lock
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, FilePermissions)
	if err != nil {
		return fmt.Errorf("creating lock file: %w", err)
	}
	defer func() {
		syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		lockFile.Close()
		os.Remove(lockPath)
	}()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	data = append(data, '\n')

	// Atomic write: temp file + rename
	tmpFile, err := os.CreateTemp(dir, "config-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if err := os.Chmod(tmpPath, FilePermissions); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("setting file permissions: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming config file: %w", err)
	}

	return nil
}

// EnsureDir creates the config directory if it does not exist.
func EnsureDir(override string) (string, error) {
	dir, _, err := Paths(override)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		return "", fmt.Errorf("creating config directory: %w", err)
	}
	return dir, nil
}

// ExpandTilde replaces a leading "~" in a path with the user's home directory.
func ExpandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, path[1:]), nil
}

// AddConnection adds a connection to the config. If a connection with the same
// name already exists, it returns an error.
func (cfg *Config) AddConnection(conn Connection) error {
	for _, c := range cfg.Connections {
		if strings.EqualFold(c.Name, conn.Name) {
			return fmt.Errorf("connection %q already exists", conn.Name)
		}
	}
	cfg.Connections = append(cfg.Connections, conn)
	return nil
}

// UpdateConnection replaces a connection by name.
func (cfg *Config) UpdateConnection(conn Connection) error {
	for i, c := range cfg.Connections {
		if strings.EqualFold(c.Name, conn.Name) {
			cfg.Connections[i] = conn
			return nil
		}
	}
	return fmt.Errorf("connection %q not found", conn.Name)
}

// RemoveConnection removes a connection by name.
func (cfg *Config) RemoveConnection(name string) error {
	for i, c := range cfg.Connections {
		if strings.EqualFold(c.Name, name) {
			cfg.Connections = append(cfg.Connections[:i], cfg.Connections[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("connection %q not found", name)
}

// FindConnection returns a connection by name (case-insensitive).
func (cfg *Config) FindConnection(name string) (*Connection, error) {
	for i, c := range cfg.Connections {
		if strings.EqualFold(c.Name, name) {
			return &cfg.Connections[i], nil
		}
	}
	return nil, fmt.Errorf("connection %q not found", name)
}

// AllAccounts returns a flat list of all accounts across all connections.
func (cfg *Config) AllAccounts() []ResolvedAccount {
	var accounts []ResolvedAccount
	for _, conn := range cfg.Connections {
		for _, acct := range conn.Accounts {
			accounts = append(accounts, ResolvedAccount{
				ConnectionName:    conn.Name,
				Account:           acct,
				ValidUntil:        conn.ValidUntil,
				RequiredPSUHeaders: conn.RequiredPSUHeaders,
			})
		}
	}
	return accounts
}

// ResolvedAccount is an account annotated with its parent connection info.
type ResolvedAccount struct {
	ConnectionName     string   `json:"connection_name"`
	Account                     // embedded
	ValidUntil         time.Time `json:"valid_until"`
	RequiredPSUHeaders []string  `json:"required_psu_headers,omitempty"`
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("EBCLI_APP_ID"); v != "" {
		cfg.AppID = v
	}
	if v := os.Getenv("EBCLI_PRIVATE_KEY"); v != "" {
		cfg.PrivateKeyPath = v
	}
}
