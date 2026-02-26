package cmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/api"
)

var banksCmd = &cobra.Command{
	Use:   "banks",
	Short: "List available banks",
	RunE: func(cmd *cobra.Command, args []string) error {
		country, _ := cmd.Flags().GetString("country")
		search, _ := cmd.Flags().GetString("search")
		psuType, _ := cmd.Flags().GetString("psu-type")

		aspsps, err := app.Client.ListASPSPs(context.Background(), country, psuType)
		if err != nil {
			return ExitWithError(ExitAPIError, "fetching banks: %v", err)
		}

		// Client-side search filter
		if search != "" {
			search = strings.ToLower(search)
			var filtered []api.ASPSPData
			for _, a := range aspsps {
				if strings.Contains(strings.ToLower(a.Name), search) {
					filtered = append(filtered, a)
				}
			}
			aspsps = filtered
		}

		// Map to output format
		var output []api.BankOutput
		for _, a := range aspsps {
			maxDays := 0
			if a.MaximumConsentValidity > 0 {
				maxDays = a.MaximumConsentValidity / 86400
			}
			output = append(output, api.BankOutput{
				Name:               a.Name,
				Country:            a.Country,
				BIC:                a.BIC,
				PSUTypes:           a.PSUTypes,
				AuthMethods:        a.AuthMethods,
				MaxConsentDays:     maxDays,
				Beta:               a.Beta,
				RequiredPSUHeaders: a.RequiredPSUHeaders,
			})
		}

		if output == nil {
			output = []api.BankOutput{}
		}

		return app.Printer.JSON(output)
	},
}

func init() {
	banksCmd.Flags().StringP("country", "c", "", "two-letter country code (required)")
	banksCmd.MarkFlagRequired("country")
	banksCmd.Flags().String("search", "", "filter banks by name (case-insensitive)")
	banksCmd.Flags().String("psu-type", "", "filter by PSU type: personal or business")
	rootCmd.AddCommand(banksCmd)
}
