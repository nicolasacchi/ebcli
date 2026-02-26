package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/api"
)

var balancesCmd = &cobra.Command{
	Use:   "balances",
	Short: "Get account balances",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		accountFlag, _ := cmd.Flags().GetString("account")

		accounts, err := resolveAccounts(accountFlag)
		if err != nil {
			return err
		}

		var output []api.BalanceOutput
		for _, ra := range accounts {
			resp, err := app.Client.GetBalances(ctx, ra.Account.UID, ra.RequiredPSUHeaders)
			if err != nil {
				app.Printer.Warn("failed to fetch balances for %s: %v", ra.Account.Alias, err)
				continue
			}
			output = append(output, api.BalanceOutput{
				Account:  ra.Account.Alias,
				IBAN:     ra.Account.IBAN,
				Balances: resp.Balances,
			})
		}

		if output == nil {
			output = []api.BalanceOutput{}
		}
		return app.Printer.JSON(output)
	},
}

func init() {
	balancesCmd.Flags().StringP("account", "a", "", "account alias, UID, or IBAN")
	balancesCmd.Flags().Bool("all", false, "fetch all accounts (default when --account not specified)")
	rootCmd.AddCommand(balancesCmd)
}
