package cmd

import (
	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/api"
)

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "List connected accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		connFilter, _ := cmd.Flags().GetString("connection")

		if app.Config == nil {
			return ExitWithError(ExitAuthError, "no config loaded. Run: ebcli config --init")
		}

		if len(app.Config.Connections) == 0 {
			return ExitWithError(ExitAuthError, "no connections configured. Run: ebcli connect")
		}

		var output []api.AccountOutput
		for _, conn := range app.Config.Connections {
			if connFilter != "" && conn.Name != connFilter {
				continue
			}
			for _, acct := range conn.Accounts {
				output = append(output, api.AccountOutput{
					UID:                acct.UID,
					IBAN:               acct.IBAN,
					Alias:              acct.Alias,
					Connection:         conn.Name,
					Currency:           acct.Currency,
					CashAccountType:    acct.CashAccountType,
					IdentificationHash: acct.IdentificationHash,
					ValidUntil:         conn.ValidUntil,
				})
			}
		}

		if output == nil {
			output = []api.AccountOutput{}
		}

		return app.Printer.JSON(output)
	},
}

func init() {
	accountsCmd.Flags().String("connection", "", "filter by connection name")
	rootCmd.AddCommand(accountsCmd)
}
