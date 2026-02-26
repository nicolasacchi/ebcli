package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/config"
)

var disconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnect a bank connection",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name, _ := cmd.Flags().GetString("name")

		conn, err := app.Config.FindConnection(name)
		if err != nil {
			return ExitWithError(ExitUserError, "%v", err)
		}

		// Revoke consent via API
		app.Printer.Info("Revoking consent for %s...", name)
		if err := app.Client.DeleteSession(ctx, conn.SessionID, conn.RequiredPSUHeaders); err != nil {
			app.Printer.Warn("could not revoke consent (session may already be expired): %v", err)
		}

		accountCount := len(conn.Accounts)

		// Remove from config
		if err := app.Config.RemoveConnection(name); err != nil {
			return ExitWithError(ExitUserError, "%v", err)
		}
		if err := config.Save(app.ConfigPath, app.Config); err != nil {
			return ExitWithError(ExitAuthError, "saving config: %v", err)
		}

		app.Printer.Info("Disconnected %s (%d accounts removed)", name, accountCount)

		output := struct {
			Disconnected    string `json:"disconnected"`
			AccountsRemoved int    `json:"accounts_removed"`
		}{
			Disconnected:    name,
			AccountsRemoved: accountCount,
		}
		return app.Printer.JSON(output)
	},
}

func init() {
	disconnectCmd.Flags().StringP("name", "n", "", "connection name (required)")
	disconnectCmd.MarkFlagRequired("name")
	rootCmd.AddCommand(disconnectCmd)
}
