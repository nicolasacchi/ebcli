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

# 2. Create an app at https://enablebanking.com/cp/applications
#    Upload the generated public key and add the redirect URL

# 3. Connect to your bank
ebcli connect --country IT --bank "ING"

# 4. Fetch data
ebcli balances
ebcli transactions --days 30
ebcli dump --all --days 90
```

## Enable Banking Setup

ebcli uses the [Enable Banking](https://enablebanking.com) API to access bank accounts via PSD2/Open Banking. You need an Enable Banking application before using ebcli.

### 1. Create an account

Sign up at [enablebanking.com](https://enablebanking.com) and go to the [Control Panel](https://enablebanking.com/cp/applications).

### 2. Create an application

Create a new application in the Control Panel. Note the **Application ID** (UUID) — you'll need it during `ebcli config --init`.

### 3. Generate keys and configure ebcli

```bash
ebcli config --init
```

The wizard will:
- Generate a 4096-bit RSA keypair (or use an existing key)
- Ask for your Application ID
- Ask for the environment (PRODUCTION or SANDBOX)
- Validate the connection

### 4. Upload the public key

Copy `~/.config/ebcli/public.pem` and paste it into the **Public Key** field of your application in the Enable Banking Control Panel.

### 5. Add the redirect URL

Add this redirect URL to your application in the Control Panel:

```
http://localhost:18271/callback
```

For production/headless servers, use your public HTTPS URL instead (see [Production Setup](#production-setup)).

### 6. Activate for production

Production applications require additional information in the Control Panel:
- Application description
- GDPR contact email
- Privacy policy URL
- Terms of service URL

Two activation paths:
- **(a) Personal use** — link your own bank accounts via the Control Panel (free)
- **(b) Commercial use** — sign a contract + KYB for full third-party access

Sandbox applications work immediately without activation.

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

Opens a browser (or prints the URL on headless machines) for bank authorization. After you authenticate, the callback is captured and a session is stored in the config. The callback server listens for 5 minutes before timing out.

Banks support different authentication methods. Most use **REDIRECT** (browser-based). Some also offer **DECOUPLED** (confirm in your banking app). Use `--auth-method` to pick a specific one when multiple are available.

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
      "iban": "IT60X...",
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
4. **IBAN** — `IT60X0542811101000000123456`

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

By default, the OAuth callback uses `http://localhost:18271/callback` — no `callback_url` in config is needed for local use. For headless or production servers, set `callback_url` in config to a public HTTPS URL that proxies to the local port (see [Production Setup](#production-setup)).

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

## Production Setup

Production Enable Banking apps require a public HTTPS callback URL and publicly accessible privacy/terms pages. This section covers how to set that up.

### Why

- The **callback URL** is where the bank redirects after authentication. For production, Enable Banking requires HTTPS — `http://localhost` won't work.
- **Privacy and Terms pages** are mandatory for production app registration.

### The `web/` directory

The `web/` directory contains everything you need:

- `nginx.conf` — serves `/privacy`, `/terms`, and proxies `/callback` to the local ebcli callback server
- `privacy.html` — template privacy policy
- `terms.html` — template terms of service

### Setup with a reverse proxy

1. Run an nginx container (or similar) serving the `web/` files:

```bash
# Example: docker run with the included config
docker run -d --name ebcli-web \
  -v $(pwd)/web/nginx.conf:/etc/nginx/conf.d/default.conf:ro \
  -v $(pwd)/web/privacy.html:/usr/share/nginx/html/privacy.html:ro \
  -v $(pwd)/web/terms.html:/usr/share/nginx/html/terms.html:ro \
  --add-host=host.docker.internal:host-gateway \
  nginx:alpine
```

2. Point your domain to the container using a TLS-terminating reverse proxy (Traefik, Caddy, etc.).

3. Set the callback URL in ebcli config:

```bash
# Edit ~/.config/ebcli/config.json
"callback_url": "https://yourdomain.com/callback"
```

4. Register in Enable Banking Control Panel:
   - Redirect URL: `https://yourdomain.com/callback`
   - Privacy URL: `https://yourdomain.com/privacy`
   - Terms URL: `https://yourdomain.com/terms`

### Headless machines

On servers without a browser, `ebcli connect` prints the authorization URL to stderr. Copy it and open in any browser — even on a different machine. The bank will redirect to your public callback URL, which proxies back to the ebcli callback server running locally.

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
