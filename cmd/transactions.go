package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/api"
	"github.com/nicolasacchi/ebcli/internal/resolver"
)

var transactionsCmd = &cobra.Command{
	Use:   "transactions",
	Short: "Get account transactions",
	RunE:  runTransactions,
}

func init() {
	transactionsCmd.Flags().StringP("account", "a", "", "account alias, UID, or IBAN")
	transactionsCmd.Flags().Bool("all", false, "fetch all accounts")
	transactionsCmd.Flags().String("from", "", "start date (YYYY-MM-DD, today, yesterday, -Nd)")
	transactionsCmd.Flags().String("to", "", "end date")
	transactionsCmd.Flags().String("days", "", "number of days back from today")
	transactionsCmd.Flags().Int("limit", 0, "max transactions to return (0=unlimited)")
	transactionsCmd.Flags().String("status", "", "transaction status: BOOK or PDNG")
	transactionsCmd.Flags().Bool("include-pending", false, "include pending transactions")
	rootCmd.AddCommand(transactionsCmd)
}

func runTransactions(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	accountFlag, _ := cmd.Flags().GetString("account")
	fromFlag, _ := cmd.Flags().GetString("from")
	toFlag, _ := cmd.Flags().GetString("to")
	daysFlag, _ := cmd.Flags().GetString("days")
	limit, _ := cmd.Flags().GetInt("limit")
	statusFlag, _ := cmd.Flags().GetString("status")
	includePending, _ := cmd.Flags().GetBool("include-pending")

	accounts, err := resolveAccounts(accountFlag)
	if err != nil {
		return err
	}

	fromDate, toDate, err := parseDateRange(fromFlag, toFlag, daysFlag)
	if err != nil {
		return ExitWithError(ExitUserError, "%v", err)
	}

	dateFrom := fromDate.Format("2006-01-02")
	dateTo := toDate.Format("2006-01-02")

	// Determine statuses to fetch
	statuses := []string{"BOOK"}
	if statusFlag != "" {
		statuses = []string{statusFlag}
	}
	if includePending {
		statuses = []string{"BOOK", "PDNG"}
	}

	var allTxns []annotatedTransaction
	for _, ra := range accounts {
		for _, status := range statuses {
			txns, err := fetchAllTransactions(ctx, ra, dateFrom, dateTo, status, limit)
			if err != nil {
				app.Printer.Warn("failed to fetch %s transactions for %s: %v", status, ra.Account.Alias, err)
				continue
			}
			allTxns = append(allTxns, txns...)
		}

		if limit > 0 && len(allTxns) >= limit {
			allTxns = allTxns[:limit]
			break
		}
	}

	if allTxns == nil {
		allTxns = []annotatedTransaction{}
	}
	return app.Printer.JSON(allTxns)
}

type annotatedTransaction struct {
	Account string `json:"account"`
	IBAN    string `json:"iban,omitempty"`
	api.Transaction
}

func fetchAllTransactions(ctx context.Context, ra resolver.Result, dateFrom, dateTo, status string, limit int) ([]annotatedTransaction, error) {
	var all []annotatedTransaction
	continuationKey := ""

	for {
		resp, err := app.Client.GetTransactions(ctx, ra.Account.UID, api.TransactionParams{
			DateFrom:          dateFrom,
			DateTo:            dateTo,
			ContinuationKey:   continuationKey,
			TransactionStatus: status,
		}, ra.RequiredPSUHeaders)
		if err != nil {
			return all, err
		}

		for _, txn := range resp.Transactions {
			all = append(all, annotatedTransaction{
				Account:     ra.Account.Alias,
				IBAN:        ra.Account.IBAN,
				Transaction: txn,
			})
			if limit > 0 && len(all) >= limit {
				return all, nil
			}
		}

		if resp.ContinuationKey == "" {
			break
		}
		continuationKey = resp.ContinuationKey
	}

	return all, nil
}
