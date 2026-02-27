package api

import "time"

// --- ASPSP (bank) types ---

// ASPSPData represents a bank from the GET /aspsps endpoint.
type ASPSPData struct {
	Name                   string         `json:"name"`
	Country                string         `json:"country"`
	Logo                   string         `json:"logo,omitempty"`
	BIC                    string         `json:"bic,omitempty"`
	PSUTypes               []string       `json:"psu_types"`
	AuthMethods            []AuthMethod   `json:"auth_methods"`
	Beta                   bool           `json:"beta"`
	MaximumConsentValidity int            `json:"maximum_consent_validity"` // seconds
	RequiredPSUHeaders     []string       `json:"required_psu_headers,omitempty"`
	Sandbox                *SandboxConfig `json:"sandbox,omitempty"`
}

// SandboxConfig holds sandbox test user credentials for a bank.
type SandboxConfig struct {
	Users []SandboxUser `json:"users,omitempty"`
}

// SandboxUser is a test credential for sandbox mode.
type SandboxUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthMethod describes a bank authentication method.
type AuthMethod struct {
	Name         string `json:"name,omitempty"`
	Title        string `json:"title,omitempty"`
	Approach     string `json:"approach"` // REDIRECT, DECOUPLED, EMBEDDED
	PSUType      string `json:"psu_type,omitempty"`
	HiddenMethod bool   `json:"hidden_method,omitempty"`
}

// --- Auth request/response ---

// AuthRequest is the body for POST /auth.
type AuthRequest struct {
	Access      AccessScope `json:"access"`
	ASPSP       ASPSPRef    `json:"aspsp"`
	State       string      `json:"state"`
	RedirectURL string      `json:"redirect_url"`
	PSUType     string      `json:"psu_type"`
	AuthMethod  string      `json:"auth_method,omitempty"`
	Language    string      `json:"language,omitempty"`
}

// AccessScope defines what data access is being requested.
type AccessScope struct {
	ValidUntil   string `json:"valid_until"` // RFC3339
	Balances     bool   `json:"balances"`
	Transactions bool   `json:"transactions"`
}

// ASPSPRef identifies a bank by name and country.
type ASPSPRef struct {
	Name    string `json:"name"`
	Country string `json:"country"`
}

// AuthResponse is the response from POST /auth.
type AuthResponse struct {
	URL             string `json:"url"`
	AuthorizationID string `json:"authorization_id"`
	PSUIDHash       string `json:"psu_id_hash,omitempty"`
}

// --- Session types ---

// SessionRequest is the body for POST /sessions.
type SessionRequest struct {
	Code string `json:"code"`
}

// SessionResponse is the response from POST /sessions.
type SessionResponse struct {
	SessionID string            `json:"session_id"`
	Accounts  []AccountResource `json:"accounts"`
	ASPSP     ASPSPRef          `json:"aspsp"`
	PSUType   string            `json:"psu_type"`
	Access    AccessInfo        `json:"access"`
}

// AccessInfo describes the session's access scope.
type AccessInfo struct {
	ValidUntil string `json:"valid_until"`
}

// AccountResource is a single account as returned by the API.
type AccountResource struct {
	UID                  string            `json:"uid"`
	AccountID            AccountID         `json:"account_id"`
	Name                 string            `json:"name,omitempty"`
	Currency             string            `json:"currency"`
	CashAccountType      string            `json:"cash_account_type"`
	Usage                string            `json:"usage,omitempty"`
	IdentificationHash   string            `json:"identification_hash"`
	IdentificationHashes []string          `json:"identification_hashes,omitempty"`
}

// AccountID holds the account identification details.
type AccountID struct {
	IBAN           string `json:"iban,omitempty"`
	Identification string `json:"identification,omitempty"`
	SchemeName     string `json:"scheme_name,omitempty"`
}

// SessionStatus is the response from GET /sessions/{session_id}.
type SessionStatus struct {
	Status       string            `json:"status"`
	Accounts     []string          `json:"accounts,omitempty"`
	AccountsData []AccountResource `json:"accounts_data,omitempty"`
	ASPSP        ASPSPRef          `json:"aspsp"`
	PSUType      string            `json:"psu_type"`
	Access       AccessInfo        `json:"access"`
	Authorized   string            `json:"authorized,omitempty"`
	Created      string            `json:"created,omitempty"`
}

// --- Account data types ---

// AccountDetails is the response from GET /accounts/{uid}/details.
type AccountDetails struct {
	UID             string    `json:"uid,omitempty"`
	IBAN            string    `json:"iban,omitempty"`
	Currency        string    `json:"currency,omitempty"`
	OwnerName       string    `json:"owner_name,omitempty"`
	Product         string    `json:"product,omitempty"`
	CashAccountType string    `json:"cash_account_type,omitempty"`
	Usage           string    `json:"usage,omitempty"`
	CreditLimit     *Amount   `json:"credit_limit,omitempty"`
}

// BalancesResponse is the response from GET /accounts/{uid}/balances.
type BalancesResponse struct {
	Balances []Balance `json:"balances"`
}

// Balance represents a single balance entry.
type Balance struct {
	Name               string `json:"name,omitempty"`
	BalanceAmount      Amount `json:"balance_amount"`
	BalanceType        string `json:"balance_type"`
	ReferenceDate      string `json:"reference_date,omitempty"`
	LastChangeDateTime string `json:"last_change_date_time,omitempty"`
}

// Amount holds a currency/amount pair. Amount is a string to preserve decimal precision.
type Amount struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

// TransactionsResponse is the response from GET /accounts/{uid}/transactions.
type TransactionsResponse struct {
	Transactions    []Transaction `json:"transactions"`
	ContinuationKey string        `json:"continuation_key,omitempty"`
}

// Transaction represents a single transaction entry.
type Transaction struct {
	TransactionID        string   `json:"transaction_id,omitempty"`
	EntryReference       string   `json:"entry_reference,omitempty"`
	BookingDate          string   `json:"booking_date,omitempty"`
	ValueDate            string   `json:"value_date,omitempty"`
	TransactionDate      string   `json:"transaction_date,omitempty"`
	TransactionAmount    Amount   `json:"transaction_amount"`
	CreditDebitIndicator string   `json:"credit_debit_indicator,omitempty"`
	Status               string   `json:"status,omitempty"`
	CreditorName         string   `json:"creditor_name,omitempty"`
	CreditorAccount      *AccountRef `json:"creditor_account,omitempty"`
	DebtorName           string   `json:"debtor_name,omitempty"`
	DebtorAccount        *AccountRef `json:"debtor_account,omitempty"`
	RemittanceInformation []string `json:"remittance_information,omitempty"`
	MerchantCategoryCode string   `json:"merchant_category_code,omitempty"`
	BalanceAfterTransaction *Balance `json:"balance_after_transaction,omitempty"`
}

// AccountRef is a reference to an account by identification.
type AccountRef struct {
	IBAN           string `json:"iban,omitempty"`
	Identification string `json:"identification,omitempty"`
	SchemeName     string `json:"scheme_name,omitempty"`
}

// TransactionParams are query parameters for the transactions endpoint.
type TransactionParams struct {
	DateFrom          string
	DateTo            string
	ContinuationKey   string
	TransactionStatus string // BOOK or PDNG
}

// --- Application ---

// ApplicationResponse is the response from GET /application.
type ApplicationResponse struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	KID          string   `json:"kid,omitempty"`
	Environment  string   `json:"environment"`
	RedirectURLs []string `json:"redirect_urls,omitempty"`
	Active       bool     `json:"active"`
	Countries    []string `json:"countries,omitempty"`
	Services     []string `json:"services,omitempty"`
}

// --- Output types for CLI ---

// BankOutput is the JSON output for the banks command.
type BankOutput struct {
	Name               string       `json:"name"`
	Country            string       `json:"country"`
	BIC                string       `json:"bic,omitempty"`
	PSUTypes           []string     `json:"psu_types"`
	AuthMethods        []AuthMethod `json:"auth_methods"`
	MaxConsentDays     int          `json:"max_consent_days"`
	Beta               bool         `json:"beta"`
	RequiredPSUHeaders []string     `json:"required_psu_headers,omitempty"`
}

// AccountOutput is the JSON output for the accounts command.
type AccountOutput struct {
	UID                string    `json:"uid"`
	IBAN               string    `json:"iban,omitempty"`
	Alias              string    `json:"alias"`
	Connection         string    `json:"connection"`
	Currency           string    `json:"currency"`
	CashAccountType    string    `json:"cash_account_type"`
	IdentificationHash string    `json:"identification_hash"`
	ValidUntil         time.Time `json:"valid_until"`
}

// BalanceOutput is the JSON output for the balances command.
type BalanceOutput struct {
	Account  string    `json:"account"`
	IBAN     string    `json:"iban,omitempty"`
	Balances []Balance `json:"balances"`
}

// DumpOutput is the JSON output for the dump command.
type DumpOutput struct {
	FetchedAt string              `json:"fetched_at"`
	Accounts  []DumpAccountOutput `json:"accounts"`
}

// DumpAccountOutput represents a single account in the dump output.
type DumpAccountOutput struct {
	Alias        string        `json:"alias"`
	IBAN         string        `json:"iban,omitempty"`
	Balances     []Balance     `json:"balances"`
	Transactions []Transaction `json:"transactions"`
}

// StatusOutput is the JSON output for the status command.
type StatusOutput struct {
	Application *ApplicationResponse  `json:"application"`
	Connections []ConnectionStatus    `json:"connections"`
}

// ConnectionStatus shows the status of a single connection.
type ConnectionStatus struct {
	Name       string    `json:"name"`
	Bank       string    `json:"bank"`
	Country    string    `json:"country"`
	Status     string    `json:"status"`
	Accounts   int       `json:"accounts"`
	ValidUntil time.Time `json:"valid_until"`
	DaysLeft   int       `json:"days_left"`
}
