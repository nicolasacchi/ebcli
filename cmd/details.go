package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/resolver"
)

var detailsCmd = &cobra.Command{
	Use:   "details",
	Short: "Get full account details from bank",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		accountFlag, _ := cmd.Flags().GetString("account")

		accounts, err := resolveAccounts(accountFlag)
		if err != nil {
			return err
		}

		var results []interface{}
		for _, ra := range accounts {
			details, err := app.Client.GetAccountDetails(ctx, ra.Account.UID, ra.RequiredPSUHeaders)
			if err != nil {
				app.Printer.Warn("failed to fetch details for %s: %v", ra.Account.Alias, err)
				continue
			}
			results = append(results, struct {
				Account string      `json:"account"`
				IBAN    string      `json:"iban,omitempty"`
				Details interface{} `json:"details"`
			}{
				Account: ra.Account.Alias,
				IBAN:    ra.Account.IBAN,
				Details: details,
			})
		}

		if results == nil {
			results = []interface{}{}
		}
		return app.Printer.JSON(results)
	},
}

func init() {
	detailsCmd.Flags().StringP("account", "a", "", "account alias, UID, or IBAN")
	rootCmd.AddCommand(detailsCmd)
}

// resolveAccounts resolves the --account flag to a list of accounts.
// If empty, returns all accounts (--all behavior).
func resolveAccounts(accountFlag string) ([]resolver.Result, error) {
	if accountFlag == "" {
		results := resolver.ResolveAll(app.Config)
		if len(results) == 0 {
			return nil, ExitWithError(ExitAuthError, "no accounts configured. Run: ebcli connect")
		}
		return results, nil
	}

	result, err := resolver.Resolve(app.Config, accountFlag)
	if err != nil {
		return nil, ExitWithError(ExitUserError, "%v", err)
	}
	return []resolver.Result{*result}, nil
}
