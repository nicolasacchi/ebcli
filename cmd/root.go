package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/api"
	"github.com/nicolasacchi/ebcli/internal/auth"
	"github.com/nicolasacchi/ebcli/internal/config"
	"github.com/nicolasacchi/ebcli/internal/output"
	"github.com/nicolasacchi/ebcli/internal/psu"
	"github.com/nicolasacchi/ebcli/internal/ratelimit"
)

const (
	ExitSuccess   = 0
	ExitUserError = 1
	ExitAPIError  = 2
	ExitAuthError = 3
)

// App holds shared dependencies for all subcommands.
type App struct {
	Config    *config.Config
	ConfigPath string
	Client    *api.Client
	Printer   *output.Printer
	RateLimit *ratelimit.Tracker
}

var (
	app         App
	flagPretty  bool
	flagCompact bool
	flagRaw     bool
	flagQuiet   bool
	flagConfig  string
	version     string
)

var rootCmd = &cobra.Command{
	Use:           "ebcli",
	Short:         "Enable Banking CLI — European bank account access",
	Long:          "Access European bank accounts via the Enable Banking API.\nOutputs structured JSON to stdout for piping into jq, claude, etc.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize printer first (always needed)
		mode := output.ModeFromFlags(flagPretty, flagCompact, flagRaw)
		app.Printer = output.NewPrinter(os.Stdout, os.Stderr, mode, flagQuiet)

		// Commands that don't need full config/client initialization
		if skipInit(cmd) {
			return nil
		}

		// Load config
		_, cfgPath, err := config.Paths(flagConfig)
		if err != nil {
			return exitError(ExitAuthError, "config path: %v", err)
		}
		app.ConfigPath = cfgPath

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return exitError(ExitAuthError, "loading config: %v", err)
		}
		app.Config = cfg

		// Commands that only need config (no API client)
		if configOnly(cmd) {
			return nil
		}

		// Validate config for API access
		if cfg.AppID == "" {
			return exitError(ExitAuthError, "app_id not configured. Run: ebcli config --init")
		}
		if cfg.PrivateKeyPath == "" {
			return exitError(ExitAuthError, "private_key_path not configured. Run: ebcli config --init")
		}

		keyPath, err := config.ExpandTilde(cfg.PrivateKeyPath)
		if err != nil {
			return exitError(ExitAuthError, "expanding key path: %v", err)
		}

		privateKey, err := auth.LoadPrivateKey(keyPath)
		if err != nil {
			return exitError(ExitAuthError, "loading private key: %v", err)
		}

		// Initialize rate limit tracker
		configDir, _, _ := config.Paths(flagConfig)
		rlTracker, err := ratelimit.NewTracker(configDir, os.Stderr)
		if err != nil {
			app.Printer.Warn("rate limit cache unavailable: %v", err)
		}
		app.RateLimit = rlTracker

		// Initialize PSU provider
		psuProvider := psu.NewProvider(version)

		// Initialize API client
		opts := []api.ClientOption{
			api.WithVersion(version),
		}
		if psuProvider != nil {
			opts = append(opts, api.WithPSUProvider(psuProvider))
		}
		if rlTracker != nil {
			opts = append(opts, api.WithRateLimiter(rlTracker))
		}

		app.Client = api.NewClient(cfg.AppID, privateKey, opts...)

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagPretty, "pretty", false, "force pretty-printed JSON output")
	rootCmd.PersistentFlags().BoolVar(&flagCompact, "compact", false, "force compact JSON output")
	rootCmd.PersistentFlags().BoolVar(&flagRaw, "raw", false, "output raw API response without transformation")
	rootCmd.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "suppress informational messages on stderr")
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "path to config file")
}

// Execute runs the root command. Called from main.
func Execute(v string) error {
	version = v
	rootCmd.Version = v

	err := rootCmd.Execute()
	if err != nil {
		// Persist rate limit cache on exit
		if app.RateLimit != nil {
			app.RateLimit.Persist()
		}
		return err
	}

	if app.RateLimit != nil {
		app.RateLimit.Persist()
	}
	return nil
}

// skipInit returns true for commands that need no config or client at all.
func skipInit(cmd *cobra.Command) bool {
	name := fullCmdName(cmd)
	// config --init creates config, doesn't need to load it
	if name == "ebcli config" {
		return true
	}
	return false
}

// configOnly returns true for commands that need config but no API client.
func configOnly(cmd *cobra.Command) bool {
	name := fullCmdName(cmd)
	return name == "ebcli accounts"
}

func fullCmdName(cmd *cobra.Command) string {
	var parts []string
	for c := cmd; c != nil; c = c.Parent() {
		parts = append([]string{c.Name()}, parts...)
	}
	return strings.Join(parts, " ")
}

type exitErr struct {
	code int
	msg  string
}

func (e *exitErr) Error() string { return e.msg }

func exitError(code int, format string, args ...interface{}) error {
	return &exitErr{code: code, msg: fmt.Sprintf(format, args...)}
}

// ExitWithError prints an error to stderr and returns an error for the exit code.
func ExitWithError(code int, format string, args ...interface{}) error {
	app.Printer.Error(format, args...)
	return &exitErr{code: code, msg: fmt.Sprintf(format, args...)}
}
