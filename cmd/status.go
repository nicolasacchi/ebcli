package cmd

import (
	"context"
	"fmt"
	"math"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/api"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connection and application status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		output := api.StatusOutput{
			Connections: []api.ConnectionStatus{},
		}

		// Check application status
		appInfo, err := app.Client.GetApplication(ctx)
		if err != nil {
			app.Printer.Warn("could not fetch application status: %v", err)
		} else {
			output.Application = appInfo
		}

		// Print application info to stderr
		if appInfo != nil {
			status := "PENDING"
			if appInfo.Active {
				status = "ACTIVE"
			}
			app.Printer.Info("Application: %s (%s, %s)", appInfo.Name, appInfo.Environment, status)
		}
		fmt.Fprintln(os.Stderr)

		// Check each connection
		if len(app.Config.Connections) == 0 {
			app.Printer.Info("No connections configured. Run: ebcli connect")
		} else {
			w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "CONNECTION\tBANK\tACCOUNTS\tVALID UNTIL\tSTATUS\tDAYS LEFT\tTODAY\n")
			fmt.Fprintf(w, "----------\t----\t--------\t-----------\t------\t---------\t-----\n")

			for _, conn := range app.Config.Connections {
				sessionStatus := "UNKNOWN"
				sessInfo, err := app.Client.GetSession(ctx, conn.SessionID)
				if err != nil {
					sessionStatus = "ERROR"
					app.Printer.Warn("could not check session for %s: %v", conn.Name, err)
				} else {
					sessionStatus = sessInfo.Status
				}

				daysLeft := int(math.Ceil(time.Until(conn.ValidUntil).Hours() / 24))
				if daysLeft < 0 {
					daysLeft = 0
					if sessionStatus == "AUTHORIZED" {
						sessionStatus = "EXPIRED"
					}
				}

				todayStr := "-"
				if conn.MaxAccessPerDay > 0 && app.RateLimit != nil {
					used, _ := app.RateLimit.DailyUsageFor(conn.Name)
					todayStr = fmt.Sprintf("%d/%d", used, conn.MaxAccessPerDay)
				}

				fmt.Fprintf(w, "%s\t%s %s\t%d\t%s\t%s\t%d\t%s\n",
					conn.Name,
					conn.ASPSPName, conn.ASPSPCountry,
					len(conn.Accounts),
					conn.ValidUntil.Format("2006-01-02"),
					sessionStatus,
					daysLeft,
					todayStr,
				)

				connStatus := api.ConnectionStatus{
					Name:       conn.Name,
					Bank:       conn.ASPSPName,
					Country:    conn.ASPSPCountry,
					Status:     sessionStatus,
					Accounts:   len(conn.Accounts),
					ValidUntil: conn.ValidUntil,
					DaysLeft:   daysLeft,
				}
				if conn.MaxAccessPerDay > 0 {
					connStatus.MaxAccessPerDay = conn.MaxAccessPerDay
					if app.RateLimit != nil {
						connStatus.DailyUsed, _ = app.RateLimit.DailyUsageFor(conn.Name)
					}
				}
				output.Connections = append(output.Connections, connStatus)
			}
			w.Flush()
		}

		return app.Printer.JSON(output)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
