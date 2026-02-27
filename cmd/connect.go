package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/nicolasacchi/ebcli/internal/api"
	"github.com/nicolasacchi/ebcli/internal/auth"
	"github.com/nicolasacchi/ebcli/internal/config"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a bank account",
	RunE:  runConnect,
}

func init() {
	connectCmd.Flags().StringP("country", "c", "", "two-letter country code (required)")
	connectCmd.MarkFlagRequired("country")
	connectCmd.Flags().StringP("bank", "b", "", "bank name (required)")
	connectCmd.MarkFlagRequired("bank")
	connectCmd.Flags().StringP("name", "n", "", "connection alias")
	connectCmd.Flags().Int("valid-days", 0, "consent validity in days (default: bank's maximum)")
	connectCmd.Flags().Int("port", auth.DefaultCallbackPort, "local callback server port")
	connectCmd.Flags().String("auth-method", "", "specific auth method name")
	rootCmd.AddCommand(connectCmd)
}

func runConnect(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	country, _ := cmd.Flags().GetString("country")
	bankName, _ := cmd.Flags().GetString("bank")
	connName, _ := cmd.Flags().GetString("name")
	validDays, _ := cmd.Flags().GetInt("valid-days")
	port, _ := cmd.Flags().GetInt("port")
	authMethodFlag, _ := cmd.Flags().GetString("auth-method")

	return doConnect(ctx, country, bankName, connName, validDays, port, authMethodFlag)
}

func doConnect(ctx context.Context, country, bankName, connName string, validDays, port int, authMethodFlag string) error {
	// Step 1: Fetch ASPSP info
	app.Printer.Info("Fetching bank information for %s in %s...", bankName, country)
	aspsps, err := app.Client.ListASPSPs(ctx, country, "personal")
	if err != nil {
		return ExitWithError(ExitAPIError, "fetching banks: %v", err)
	}

	var aspsp *api.ASPSPData
	for i, a := range aspsps {
		if strings.EqualFold(a.Name, bankName) {
			aspsp = &aspsps[i]
			break
		}
	}
	if aspsp == nil {
		// Try substring match
		var matches []string
		for _, a := range aspsps {
			if strings.Contains(strings.ToLower(a.Name), strings.ToLower(bankName)) {
				matches = append(matches, a.Name)
			}
		}
		if len(matches) > 0 {
			return ExitWithError(ExitUserError, "bank %q not found. Did you mean: %s?", bankName, strings.Join(matches, ", "))
		}
		return ExitWithError(ExitUserError, "bank %q not found in %s", bankName, country)
	}

	// Step 2: Select auth method
	selectedMethod, approach, err := selectAuthMethod(aspsp, authMethodFlag)
	if err != nil {
		return err
	}

	if approach == "EMBEDDED" {
		return ExitWithError(ExitUserError, "EMBEDDED auth is not supported. Use a bank with REDIRECT or DECOUPLED auth, or specify --auth-method")
	}

	// Step 3: Calculate valid_until
	maxSeconds := aspsp.MaximumConsentValidity
	if maxSeconds <= 0 {
		maxSeconds = 90 * 86400 // default 90 days
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

	// Step 4: Generate state
	state := uuid.New().String()

	// Step 5: POST /auth
	callbackURL := auth.CallbackURL(port, app.Config.CallbackURL)
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

	app.Printer.Info("Starting authorization...")
	authResp, err := app.Client.Authorize(ctx, authReq)
	if err != nil {
		return ExitWithError(ExitAPIError, "authorization request failed: %v", err)
	}

	// Step 6: Handle auth approach
	if approach == "DECOUPLED" {
		app.Printer.Info("Please complete authorization in your banking app...")
		app.Printer.Info("Waiting for confirmation (timeout: 5 minutes)...")
	} else {
		app.Printer.Info("Open this URL to authorize:")
		fmt.Fprintln(os.Stderr, authResp.URL)
		_ = openBrowser(authResp.URL)
	}

	// Wait for callback
	result, err := auth.ListenForCallback(ctx, port, state)
	if err != nil {
		if result != nil && result.Error != "" {
			return ExitWithError(ExitAPIError, "bank authorization failed: %s — %s", result.Error, result.ErrorDescription)
		}
		return ExitWithError(ExitAPIError, "authorization callback: %v", err)
	}

	// Step 7: POST /sessions
	app.Printer.Info("Creating session...")
	session, err := app.Client.CreateSession(ctx, result.Code)
	if err != nil {
		return ExitWithError(ExitAPIError, "creating session: %v", err)
	}

	// Step 8: Build connection
	if connName == "" {
		connName = generateConnectionName(aspsp.Name, app.Config)
	}

	accounts := mapAccounts(session.Accounts, aspsp.Name, app.Config)

	conn := config.Connection{
		Name:                      connName,
		ASPSPCountry:              aspsp.Country,
		ASPSPName:                 aspsp.Name,
		SessionID:                 session.SessionID,
		Accounts:                  accounts,
		ConnectedAt:               time.Now(),
		ValidUntil:                validUntil,
		MaxConsentValiditySeconds: aspsp.MaximumConsentValidity,
		RequiredPSUHeaders:        aspsp.RequiredPSUHeaders,
	}

	if err := app.Config.AddConnection(conn); err != nil {
		return ExitWithError(ExitUserError, "%v", err)
	}

	if err := config.Save(app.ConfigPath, app.Config); err != nil {
		return ExitWithError(ExitAuthError, "saving config: %v", err)
	}

	app.Printer.Info("Connected! %d account(s) discovered", len(accounts))
	for _, acct := range accounts {
		app.Printer.Info("  %s: %s (%s)", acct.Alias, acct.IBAN, acct.Currency)
	}

	return app.Printer.JSON(conn)
}

func selectAuthMethod(aspsp *api.ASPSPData, preferred string) (methodName string, approach string, err error) {
	methods := aspsp.AuthMethods
	if len(methods) == 0 {
		return "", "REDIRECT", nil // default
	}

	// If user specified a method, find it
	if preferred != "" {
		for _, m := range methods {
			if strings.EqualFold(m.Name, preferred) {
				return m.Name, m.Approach, nil
			}
		}
		return "", "", ExitWithError(ExitUserError, "auth method %q not found for %s", preferred, aspsp.Name)
	}

	// Filter out EMBEDDED methods
	var usable []api.AuthMethod
	for _, m := range methods {
		if m.Approach != "EMBEDDED" {
			usable = append(usable, m)
		}
	}

	if len(usable) == 0 {
		return "", "", ExitWithError(ExitUserError, "no supported auth methods for %s (only EMBEDDED available)", aspsp.Name)
	}

	if len(usable) == 1 {
		return usable[0].Name, usable[0].Approach, nil
	}

	// Multiple methods — show on stderr and read from /dev/tty
	app.Printer.Info("Multiple auth methods available:")
	for i, m := range usable {
		app.Printer.Info("  [%d] %s (%s)", i+1, m.Name, m.Approach)
	}

	tty, err := os.Open("/dev/tty")
	if err != nil {
		// Fallback: use first method
		app.Printer.Warn("Cannot open /dev/tty, using first method: %s", usable[0].Name)
		return usable[0].Name, usable[0].Approach, nil
	}
	defer tty.Close()

	app.Printer.Info("Select method [1-%d]: ", len(usable))
	var choice int
	fmt.Fscan(tty, &choice)

	if choice < 1 || choice > len(usable) {
		return usable[0].Name, usable[0].Approach, nil
	}
	return usable[choice-1].Name, usable[choice-1].Approach, nil
}

func generateConnectionName(bankName string, cfg *config.Config) string {
	base := strings.ToLower(strings.ReplaceAll(bankName, " ", "-"))
	name := base
	suffix := 1
	for {
		exists := false
		for _, c := range cfg.Connections {
			if strings.EqualFold(c.Name, name) {
				exists = true
				break
			}
		}
		if !exists {
			return name
		}
		suffix++
		name = fmt.Sprintf("%s-%d", base, suffix)
	}
}

func mapAccounts(apiAccounts []api.AccountResource, bankName string, cfg *config.Config) []config.Account {
	existingAliases := make(map[string]bool)
	for _, conn := range cfg.Connections {
		for _, acct := range conn.Accounts {
			existingAliases[strings.ToLower(acct.Alias)] = true
		}
	}

	var accounts []config.Account
	for _, a := range apiAccounts {
		alias := generateAlias(bankName, a.Currency, existingAliases)
		existingAliases[strings.ToLower(alias)] = true

		accounts = append(accounts, config.Account{
			UID:                a.UID,
			IBAN:               a.AccountID.IBAN,
			Alias:              alias,
			Currency:           a.Currency,
			IdentificationHash: a.IdentificationHash,
			CashAccountType:    a.CashAccountType,
		})
	}
	return accounts
}

func generateAlias(bankName, currency string, existing map[string]bool) string {
	base := strings.ToLower(strings.ReplaceAll(bankName, " ", "-")) + "-" + strings.ToLower(currency)
	alias := base
	suffix := 1
	for existing[strings.ToLower(alias)] {
		suffix++
		alias = fmt.Sprintf("%s-%d", base, suffix)
	}
	return alias
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Start()
}
