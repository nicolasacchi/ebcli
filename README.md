# ebcli

CLI tool for accessing European bank accounts via the [Enable Banking](https://enablebanking.com) API.

Outputs structured JSON to stdout. Human messages go to stderr. Designed to pipe into `jq`, `claude`, or any tool that reads JSON.

```bash
ebcli dump --all --days 30 | claude "analyze my spending patterns"
ebcli balances | jq '.[].balances[].balance_amount'
ebcli transactions --days 7 --account ing-eur | jq '.[] | select(.transaction_amount.amount | tonumber > 100)'
```

## Install

```bash
# From source
go install github.com/nicolasacchi/ebcli@latest

# Or clone and build
git clone https://github.com/nicolasacchi/ebcli.git
cd ebcli
make build      # produces ./ebcli
make install    # installs to $GOPATH/bin
```

## Quick Start

```bash
# 1. Set up config (generates RSA keypair, asks for app ID)
ebcli config --init

# 2. Upload the generated public key at https://enablebanking.com/cp/applications

# 3. Connect to your bank
ebcli connect --country IT --bank "ING"

# 4. Fetch data
ebcli balances
ebcli transactions --days 30
ebcli dump --all --days 90
```

## Commands

### config

Manage configuration.

```bash
ebcli config --init    # interactive setup wizard
ebcli config           # show current config as JSON
```

The `--init` wizard generates a 4096-bit RSA keypair, asks for your Enable Banking application ID, and validates the connection.

### banks

List available banks in a country.

```bash
ebcli banks --country FI
ebcli banks --country IT --search "ING"
ebcli banks --country DE --psu-type personal
```

| Flag | Short | Description |
|------|-------|-------------|
| `--country` | `-c` | Two-letter country code (required) |
| `--search` | | Filter by name (case-insensitive) |
| `--psu-type` | | Filter: `personal` or `business` |

### connect

Connect to a bank account via OAuth.

```bash
ebcli connect --country IT --bank "ING"
ebcli connect --country FI --bank "Nordea" --name nordea-main --valid-days 90
```

Opens a browser (or prints the URL on headless machines) for bank authorization. After you authenticate, the callback is captured and a session is stored in the config.

| Flag | Short | Description |
|------|-------|-------------|
| `--country` | `-c` | Country code (required) |
| `--bank` | `-b` | Bank name (required) |
| `--name` | `-n` | Custom connection alias |
| `--valid-days` | | Consent duration in days (default: bank's maximum) |
| `--port` | | Callback server port (default: 18271) |
| `--auth-method` | | Specific auth method name |

### accounts

List all connected accounts.

```bash
ebcli accounts
ebcli accounts --connection ing
```

### balances

Fetch account balances.

```bash
ebcli balances                        # all accounts
ebcli balances --account ing-eur      # specific account
```

When `--account` is not specified, fetches all accounts.

| Flag | Short | Description |
|------|-------|-------------|
| `--account` | `-a` | Account alias, UID, or IBAN |
| `--all` | | Explicitly fetch all accounts |

### transactions

Fetch account transactions.

```bash
ebcli transactions --days 30
ebcli transactions --account ing-eur --from 2024-01-01 --to 2024-12-31
ebcli transactions --days 7 --include-pending
ebcli transactions --days 90 --limit 50
```

Default date range: last 30 days.

| Flag | Short | Description |
|------|-------|-------------|
| `--account` | `-a` | Account alias, UID, or IBAN |
| `--all` | | Fetch all accounts |
| `--from` | | Start date |
| `--to` | | End date |
| `--days` | | Days back from today |
| `--limit` | | Max transactions (0 = unlimited) |
| `--status` | | Filter: `BOOK` or `PDNG` |
| `--include-pending` | | Include pending transactions |

### dump

Fetch balances and transactions together in a single JSON object. Designed for piping to LLMs.

```bash
ebcli dump --all --days 30
ebcli dump --account ing-eur --days 90 | claude "categorize these expenses"
```

Output format:
```json
{
  "fetched_at": "2026-02-27T22:30:00Z",
  "accounts": [
    {
      "alias": "ing-eur",
      "iban": "IT84C...",
      "balances": [...],
      "transactions": [...]
    }
  ]
}
```

| Flag | Short | Description |
|------|-------|-------------|
| `--account` | `-a` | Account alias, UID, or IBAN |
| `--all` | | All accounts (default when --account not specified) |
| `--from` | | Start date |
| `--to` | | End date |
| `--days` | | Days back from today |

### details

Get full account details from the bank.

```bash
ebcli details --account ing-eur
ebcli details    # all accounts
```

### status

Show application and connection health.

```bash
ebcli status
```

Outputs a human-readable table to stderr and JSON to stdout. Shows app status, connection states, account counts, and days until consent expiry.

### disconnect

Revoke a bank connection.

```bash
ebcli disconnect --name ing
```

Deletes the session at the API and removes the connection from config.

### reconnect

Refresh an expired or expiring connection. Preserves account aliases by matching via identification hash.

```bash
ebcli reconnect --name ing
ebcli reconnect --name ing --valid-days 180
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--pretty` | Force pretty-printed JSON |
| `--compact` | Force compact JSON (single line) |
| `--raw` | Output raw API response without transformation |
| `--quiet` | Suppress stderr messages |
| `--config` | Path to config file |

**Auto mode** (default): pretty JSON when stdout is a terminal, compact when piped.

## Date Formats

All date flags (`--from`, `--to`) accept:

| Format | Example | Description |
|--------|---------|-------------|
| `YYYY-MM-DD` | `2024-01-15` | Exact date |
| `today` | | Current date |
| `yesterday` | | Previous day |
| `-Nd` | `-7d`, `-30d` | N days ago |

## Account Resolution

The `--account` flag accepts multiple identifiers, resolved in order:

1. **Alias** — `ing-eur` (auto-generated as `bankname-currency-N`)
2. **UID** — `9594e67d-faf8-4aee-811f-964bdecf4d66`
3. **UID prefix** — `9594e` (minimum 4 characters)
4. **IBAN** — `IT84C0347501605CC0010548351`

All matching is case-insensitive.

## Configuration

Config file: `~/.config/ebcli/config.json`

```json
{
  "app_id": "a8bfb6c9-...",
  "private_key_path": "/home/user/.config/ebcli/private.pem",
  "environment": "PRODUCTION",
  "callback_url": "https://example.com/callback",
  "connections": [...]
}
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `EBCLI_APP_ID` | Override app ID |
| `EBCLI_PRIVATE_KEY` | Override private key path |
| `EBCLI_CONFIG` | Override config file path |

### Callback URL

By default, the OAuth callback uses `http://localhost:18271/callback`. For headless servers, set `callback_url` in config to a public HTTPS URL that proxies to the local port.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | User error (bad flags, ambiguous account) |
| 2 | API error (bank error, rate limit, network) |
| 3 | Auth/config error (missing key, expired session) |

## Output Convention

- **stdout**: Only valid JSON. Safe to pipe.
- **stderr**: Human-readable messages (progress, warnings, errors).
- `--quiet` suppresses all stderr output.
- `--raw` outputs the API response verbatim.

## Headless / Production Setup

For servers without a browser:

1. Run `ebcli config --init` to generate keys
2. Register your app at https://enablebanking.com/cp/applications
3. Set up a reverse proxy for the callback URL (e.g., nginx behind Traefik)
4. Set `callback_url` in config to the public URL
5. Run `ebcli connect` — copy the printed URL and open in any browser

The `web/` directory contains an example nginx config and privacy/terms pages for Enable Banking production requirements.

## Build

```bash
make build        # build for current platform
make test         # run tests
make test-v       # run tests (verbose)
make lint         # run golangci-lint
make release      # cross-compile for linux/darwin amd64/arm64
make clean        # remove build artifacts
```

## Dependencies

- [cobra](https://github.com/spf13/cobra) — CLI framework
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — RS256 JWT generation
- [fatih/color](https://github.com/fatih/color) — Colored stderr output
- [google/uuid](https://github.com/google/uuid) — UUID generation

Everything else uses the Go standard library.
