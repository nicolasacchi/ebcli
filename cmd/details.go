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

		accounts = checkDailyLimits(accounts)
		if len(accounts) == 0 {
			return ExitWithError(ExitAPIError, "all accounts skipped due to daily limits")
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

		recordDailyAccess(accounts)
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

// checkDailyLimits filters out accounts whose connection has exceeded its daily
// access limit. Returns the allowed accounts and warns about skipped ones.
func checkDailyLimits(accounts []resolver.Result) []resolver.Result {
	if app.RateLimit == nil {
		return accounts
	}

	checked := make(map[string]bool)   // connection name -> allowed
	var allowed []resolver.Result

	for _, ra := range accounts {
		connName := ra.Connection.Name
		maxPerDay := ra.Connection.MaxAccessPerDay

		if ok, seen := checked[connName]; seen {
			if ok {
				allowed = append(allowed, ra)
			}
			continue
		}

		if err := app.RateLimit.CheckDaily(connName, maxPerDay); err != nil {
			app.Printer.Warn("%v", err)
			checked[connName] = false
			continue
		}

		checked[connName] = true
		allowed = append(allowed, ra)
	}

	return allowed
}

// recordDailyAccess records one daily access per connection for the given accounts.
// Should be called after successful data fetch. Deduplicates by connection name.
func recordDailyAccess(accounts []resolver.Result) {
	if app.RateLimit == nil {
		return
	}

	recorded := make(map[string]bool)
	for _, ra := range accounts {
		connName := ra.Connection.Name
		if recorded[connName] {
			continue
		}
		if ra.Connection.MaxAccessPerDay > 0 {
			app.RateLimit.RecordDaily(connName, ra.Connection.MaxAccessPerDay)
			recorded[connName] = true
		}
	}
}
