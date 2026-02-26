package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/api"
	"github.com/nicolasacchi/ebcli/internal/auth"
	"github.com/nicolasacchi/ebcli/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ebcli configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		initFlag, _ := cmd.Flags().GetBool("init")
		if initFlag {
			return runConfigInit(cmd)
		}
		return runConfigShow(cmd)
	},
}

func init() {
	configCmd.Flags().Bool("init", false, "run interactive setup wizard")
	rootCmd.AddCommand(configCmd)
}

func runConfigInit(cmd *cobra.Command) error {
	reader := bufio.NewReader(os.Stdin)

	dir, err := config.EnsureDir(flagConfig)
	if err != nil {
		return ExitWithError(ExitAuthError, "creating config directory: %v", err)
	}
	app.Printer.Info("Config directory: %s", dir)

	cfg := &config.Config{}

	// Key handling
	app.Printer.Info("Do you already have a private key from Enable Banking? [y/N]")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer == "y" || answer == "yes" {
		app.Printer.Info("Enter path to your .pem private key file:")
		keyPath, _ := reader.ReadString('\n')
		keyPath = strings.TrimSpace(keyPath)

		expanded, err := config.ExpandTilde(keyPath)
		if err != nil {
			return ExitWithError(ExitAuthError, "expanding path: %v", err)
		}

		if _, err := auth.LoadPrivateKey(expanded); err != nil {
			return ExitWithError(ExitAuthError, "loading private key: %v", err)
		}
		app.Printer.Info("Private key loaded successfully")
		cfg.PrivateKeyPath = keyPath

		// Try to extract app_id from filename (browser keys are named <app_id>.pem)
		base := filepath.Base(keyPath)
		if strings.HasSuffix(base, ".pem") {
			candidate := strings.TrimSuffix(base, ".pem")
			if len(candidate) == 36 && strings.Count(candidate, "-") == 4 {
				app.Printer.Info("Detected app_id from filename: %s", candidate)
				cfg.AppID = candidate
			}
		}
	} else {
		privPath := filepath.Join(dir, "private.pem")
		pubPath := filepath.Join(dir, "public.pem")

		privFile, err := os.OpenFile(privPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return ExitWithError(ExitAuthError, "creating private key file: %v", err)
		}

		pubFile, err := os.OpenFile(pubPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			privFile.Close()
			return ExitWithError(ExitAuthError, "creating public key file: %v", err)
		}

		app.Printer.Info("Generating 4096-bit RSA keypair (this may take a moment)...")
		if err := auth.GenerateKeyPair(privFile, pubFile); err != nil {
			privFile.Close()
			pubFile.Close()
			return ExitWithError(ExitAuthError, "generating keypair: %v", err)
		}
		privFile.Close()
		pubFile.Close()

		app.Printer.Info("Private key saved to: %s", privPath)
		app.Printer.Info("Public key saved to: %s", pubPath)
		app.Printer.Info("")
		app.Printer.Info("Upload the public key at: https://enablebanking.com/cp/applications")
		cfg.PrivateKeyPath = privPath
	}

	// App ID
	if cfg.AppID == "" {
		app.Printer.Info("Enter your application ID (UUID from Enable Banking):")
		appID, _ := reader.ReadString('\n')
		cfg.AppID = strings.TrimSpace(appID)
	}
	if cfg.AppID == "" {
		return ExitWithError(ExitAuthError, "app_id is required")
	}

	// Environment
	app.Printer.Info("Environment [PRODUCTION/sandbox] (default: PRODUCTION):")
	env, _ := reader.ReadString('\n')
	env = strings.TrimSpace(strings.ToUpper(env))
	if env == "" || env == "PRODUCTION" {
		cfg.Environment = "PRODUCTION"
	} else {
		cfg.Environment = "SANDBOX"
	}

	// Redirect URL reminder
	fmt.Fprintln(os.Stderr)
	app.Printer.Info("Add this redirect URL to your Enable Banking application:")
	app.Printer.Info("  http://localhost:18271/callback")
	fmt.Fprintln(os.Stderr)

	if cfg.Environment == "PRODUCTION" {
		app.Printer.Info("For production apps, you also need:")
		app.Printer.Info("  - Application description")
		app.Printer.Info("  - GDPR email")
		app.Printer.Info("  - Privacy URL")
		app.Printer.Info("  - Terms URL")
		fmt.Fprintln(os.Stderr)
		app.Printer.Info("Activation paths:")
		app.Printer.Info("  (a) Link your own accounts via Control Panel -> free personal use")
		app.Printer.Info("  (b) Sign contract + KYB -> full commercial access")
	}

	// Save config
	cfg.Connections = []config.Connection{}
	_, cfgPath, err := config.Paths(flagConfig)
	if err != nil {
		return ExitWithError(ExitAuthError, "resolving config path: %v", err)
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return ExitWithError(ExitAuthError, "saving config: %v", err)
	}
	app.Printer.Info("Configuration saved to: %s", cfgPath)

	// Validate
	app.Printer.Info("Validating connection to Enable Banking...")
	keyPath, err := config.ExpandTilde(cfg.PrivateKeyPath)
	if err != nil {
		app.Printer.Warn("Could not expand key path: %v", err)
		return nil
	}
	privateKey, err := auth.LoadPrivateKey(keyPath)
	if err != nil {
		app.Printer.Warn("Could not load key for validation: %v", err)
		return nil
	}

	client := api.NewClient(cfg.AppID, privateKey)
	appInfo, err := client.GetApplication(context.Background())
	if err != nil {
		app.Printer.Warn("Could not validate app: %v", err)
		app.Printer.Warn("Config saved. Validate later with: ebcli status")
		return nil
	}

	if appInfo.Active {
		app.Printer.Info("Application '%s' is ACTIVE (%s)", appInfo.Name, appInfo.Environment)
	} else {
		app.Printer.Warn("Application '%s' is PENDING — link accounts at https://enablebanking.com/cp/applications", appInfo.Name)
	}

	return nil
}

func runConfigShow(cmd *cobra.Command) error {
	_, cfgPath, err := config.Paths(flagConfig)
	if err != nil {
		return ExitWithError(ExitAuthError, "resolving config path: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return ExitWithError(ExitAuthError, "loading config: %v", err)
	}

	output := struct {
		AppID          string `json:"app_id"`
		PrivateKeyPath string `json:"private_key_path"`
		Environment    string `json:"environment"`
		ConfigPath     string `json:"config_path"`
		Connections    int    `json:"connections"`
	}{
		AppID:          cfg.AppID,
		PrivateKeyPath: cfg.PrivateKeyPath,
		Environment:    cfg.Environment,
		ConfigPath:     cfgPath,
		Connections:    len(cfg.Connections),
	}

	return app.Printer.JSON(output)
}
