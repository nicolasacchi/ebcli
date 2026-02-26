package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/api"
	"github.com/nicolasacchi/ebcli/internal/auth"
	"github.com/nicolasacchi/ebcli/internal/config"
)

var reconnectCmd = &cobra.Command{
	Use:   "reconnect",
	Short: "Reconnect an expired or expiring connection",
	RunE:  runReconnect,
}

func init() {
	reconnectCmd.Flags().StringP("name", "n", "", "connection name (required)")
	reconnectCmd.MarkFlagRequired("name")
	reconnectCmd.Flags().Int("valid-days", 0, "consent validity in days")
	reconnectCmd.Flags().Int("port", auth.DefaultCallbackPort, "local callback server port")
	rootCmd.AddCommand(reconnectCmd)
}

func runReconnect(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	name, _ := cmd.Flags().GetString("name")
	validDays, _ := cmd.Flags().GetInt("valid-days")
	port, _ := cmd.Flags().GetInt("port")

	oldConn, err := app.Config.FindConnection(name)
	if err != nil {
		return ExitWithError(ExitUserError, "%v", err)
	}

	// Save old accounts for identification_hash matching
	oldAccountsByHash := make(map[string]config.Account)
	for _, acct := range oldConn.Accounts {
		if acct.IdentificationHash != "" {
			oldAccountsByHash[acct.IdentificationHash] = acct
		}
	}

	// Fetch ASPSP info
	app.Printer.Info("Fetching bank information for %s...", oldConn.ASPSPName)
	aspsps, err := app.Client.ListASPSPs(ctx, oldConn.ASPSPCountry, "personal")
	if err != nil {
		return ExitWithError(ExitAPIError, "fetching banks: %v", err)
	}

	var aspsp *api.ASPSPData
	for i, a := range aspsps {
		if strings.EqualFold(a.Name, oldConn.ASPSPName) {
			aspsp = &aspsps[i]
			break
		}
	}
	if aspsp == nil {
		return ExitWithError(ExitAPIError, "bank %q no longer available", oldConn.ASPSPName)
	}

	// Select first non-EMBEDDED auth method
	var selectedMethod string
	var approach string
	for _, m := range aspsp.AuthMethods {
		if m.Approach != "EMBEDDED" {
			selectedMethod = m.Name
			approach = m.Approach
			break
		}
	}

	// Calculate valid_until
	maxSeconds := aspsp.MaximumConsentValidity
	if maxSeconds <= 0 {
		maxSeconds = oldConn.MaxConsentValiditySeconds
	}
	if maxSeconds <= 0 {
		maxSeconds = 90 * 86400
	}

	var validUntil time.Time
	if validDays > 0 {
		requested := validDays * 86400
		if requested > maxSeconds {
			app.Printer.Warn("Requested %d days exceeds bank maximum (%d days), capping", validDays, maxSeconds/86400)
			validUntil = time.Now().Add(time.Duration(maxSeconds) * time.Second)
		} else {
			validUntil = time.Now().AddDate(0, 0, validDays)
		}
	} else {
		validUntil = time.Now().Add(time.Duration(maxSeconds) * time.Second)
	}

	// New auth flow
	state := uuid.New().String()
	callbackURL := auth.CallbackURL(port)

	authReq := &api.AuthRequest{
		Access: api.AccessScope{
			ValidUntil:   validUntil.Format(time.RFC3339),
			Balances:     true,
			Transactions: true,
		},
		ASPSP:       api.ASPSPRef{Name: aspsp.Name, Country: aspsp.Country},
		State:       state,
		RedirectURL: callbackURL,
		PSUType:     "personal",
		AuthMethod:  selectedMethod,
		Language:    "en",
	}

	app.Printer.Info("Starting re-authorization for %s...", name)
	authResp, err := app.Client.Authorize(ctx, authReq)
	if err != nil {
		return ExitWithError(ExitAPIError, "authorization request failed: %v", err)
	}

	if approach == "DECOUPLED" {
		app.Printer.Info("Please complete authorization in your banking app...")
	} else {
		app.Printer.Info("Opening bank authorization page...")
		if err := openBrowser(authResp.URL); err != nil {
			app.Printer.Warn("Could not open browser. Visit: %s", authResp.URL)
		}
	}

	result, err := auth.ListenForCallback(ctx, port, state)
	if err != nil {
		if result != nil && result.Error != "" {
			return ExitWithError(ExitAPIError, "authorization failed: %s — %s", result.Error, result.ErrorDescription)
		}
		return ExitWithError(ExitAPIError, "authorization callback: %v", err)
	}

	app.Printer.Info("Creating session...")
	session, err := app.Client.CreateSession(ctx, result.Code)
	if err != nil {
		return ExitWithError(ExitAPIError, "creating session: %v", err)
	}

	// Match accounts by identification_hash
	existingAliases := make(map[string]bool)
	for _, conn := range app.Config.Connections {
		if conn.Name == name {
			continue // skip the connection being reconnected
		}
		for _, acct := range conn.Accounts {
			existingAliases[strings.ToLower(acct.Alias)] = true
		}
	}

	var newAccounts []config.Account
	for _, apiAcct := range session.Accounts {
		var alias string
		if old, ok := oldAccountsByHash[apiAcct.IdentificationHash]; ok {
			alias = old.Alias
			app.Printer.Info("Matched account %s -> %s", apiAcct.AccountID.IBAN, alias)
		} else {
			alias = generateAlias(aspsp.Name, apiAcct.Currency, existingAliases)
			app.Printer.Warn("New account discovered: %s -> %s", apiAcct.AccountID.IBAN, alias)
		}
		existingAliases[strings.ToLower(alias)] = true

		newAccounts = append(newAccounts, config.Account{
			UID:                apiAcct.UID,
			IBAN:               apiAcct.AccountID.IBAN,
			Alias:              alias,
			Currency:           apiAcct.Currency,
			IdentificationHash: apiAcct.IdentificationHash,
			CashAccountType:    apiAcct.CashAccountType,
		})
	}

	// Check for lost accounts
	for hash, oldAcct := range oldAccountsByHash {
		found := false
		for _, apiAcct := range session.Accounts {
			if apiAcct.IdentificationHash == hash {
				found = true
				break
			}
		}
		if !found {
			app.Printer.Warn("Account %s (%s) no longer available after reconnect", oldAcct.Alias, oldAcct.IBAN)
		}
	}

	// Update connection
	updatedConn := config.Connection{
		Name:                      name,
		ASPSPCountry:              aspsp.Country,
		ASPSPName:                 aspsp.Name,
		SessionID:                 session.SessionID,
		Accounts:                  newAccounts,
		ConnectedAt:               time.Now(),
		ValidUntil:                validUntil,
		MaxConsentValiditySeconds: aspsp.MaximumConsentValidity,
		RequiredPSUHeaders:        aspsp.RequiredPSUHeaders,
	}

	if err := app.Config.UpdateConnection(updatedConn); err != nil {
		return ExitWithError(ExitUserError, "%v", err)
	}
	if err := config.Save(app.ConfigPath, app.Config); err != nil {
		return ExitWithError(ExitAuthError, "saving config: %v", err)
	}

	app.Printer.Info("Reconnected! %d account(s)", len(newAccounts))
	return app.Printer.JSON(updatedConn)
}

func init() {
	// flags already registered above
	_ = fmt.Sprintf // ensure fmt is used
}
