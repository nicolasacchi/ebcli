package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/api"
)

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump balances and transactions (feed to LLM)",
	Long:  "Fetch both balances and transactions in a single JSON object.\nDesigned for: ebcli dump --all --days 30 | claude \"analyze spending\"",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		accountFlag, _ := cmd.Flags().GetString("account")
		fromFlag, _ := cmd.Flags().GetString("from")
		toFlag, _ := cmd.Flags().GetString("to")
		daysFlag, _ := cmd.Flags().GetString("days")

		accounts, err := resolveAccounts(accountFlag)
		if err != nil {
			return err
		}

		accounts = checkDailyLimits(accounts)
		if len(accounts) == 0 {
			return ExitWithError(ExitAPIError, "all accounts skipped due to daily limits")
		}

		fromDate, toDate, err := parseDateRange(fromFlag, toFlag, daysFlag)
		if err != nil {
			return ExitWithError(ExitUserError, "%v", err)
		}

		dateFrom := fromDate.Format("2006-01-02")
		dateTo := toDate.Format("2006-01-02")

		output := api.DumpOutput{
			FetchedAt: time.Now().Format(time.RFC3339),
			Accounts:  []api.DumpAccountOutput{},
		}

		for _, ra := range accounts {
			app.Printer.Info("Fetching data for %s...", ra.Account.Alias)

			// Fetch balances
			var balances []api.Balance
			balResp, err := app.Client.GetBalances(ctx, ra.Account.UID, ra.RequiredPSUHeaders)
			if err != nil {
				app.Printer.Warn("failed to fetch balances for %s: %v", ra.Account.Alias, err)
			} else {
				balances = balResp.Balances
			}

			// Fetch transactions (all pages)
			var transactions []api.Transaction
			continuationKey := ""
			for {
				txnResp, err := app.Client.GetTransactions(ctx, ra.Account.UID, api.TransactionParams{
					DateFrom:          dateFrom,
					DateTo:            dateTo,
					ContinuationKey:   continuationKey,
					TransactionStatus: "BOOK",
				}, ra.RequiredPSUHeaders)
				if err != nil {
					app.Printer.Warn("failed to fetch transactions for %s: %v", ra.Account.Alias, err)
					break
				}
				transactions = append(transactions, txnResp.Transactions...)
				if txnResp.ContinuationKey == "" {
					break
				}
				continuationKey = txnResp.ContinuationKey
			}

			if balances == nil {
				balances = []api.Balance{}
			}
			if transactions == nil {
				transactions = []api.Transaction{}
			}

			output.Accounts = append(output.Accounts, api.DumpAccountOutput{
				Alias:        ra.Account.Alias,
				IBAN:         ra.Account.IBAN,
				Balances:     balances,
				Transactions: transactions,
			})
		}

		recordDailyAccess(accounts)
		return app.Printer.JSON(output)
	},
}

func init() {
	dumpCmd.Flags().StringP("account", "a", "", "account alias, UID, or IBAN")
	dumpCmd.Flags().Bool("all", false, "all accounts (default when --account not specified)")
	dumpCmd.Flags().String("from", "", "start date")
	dumpCmd.Flags().String("to", "", "end date")
	dumpCmd.Flags().String("days", "", "days back from today")
	rootCmd.AddCommand(dumpCmd)
}
