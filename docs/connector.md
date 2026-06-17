# RBHU AIS connector for Claude

A read-only MCP connector that exposes the Raiffeisen Hungary AIS APIs to
Claude. Same tools over two transports:

- **stdio** (`cmd/rbhu-connector`) — local, for Claude Desktop / Claude Code.
- **Streamable HTTP** (`cmd/rbhu-connector-http`) — remote, for a claude.ai
  custom connector.

Read-only by design: it can create an AIS consent, complete SCA, and read
accounts/balances/transactions. The OAuth scope is `AISP` only — no payment or
write operation is reachable.

## Tools

| Tool | Purpose |
|---|---|
| `create_consent` | Create an AIS consent for IBANs; returns `consent_id` + `authorize_url`. |
| `submit_authorization_code` | Exchange the `?code=` from the SCA redirect for a token. |
| `authorize_in_browser` | Local only: open SCA in a browser and auto-capture the code. |
| `list_accounts` | List accounts (current `CACC` and savings `SVGS`). |
| `get_balances` | Balances for one account. |
| `get_transactions` | Transactions for one account (`booked`/`pending`/`both`). |

## Build

```sh
make connectors            # builds bin/rbhu-connector and bin/rbhu-connector-http
```

Credentials are loaded from `secrets/.env` + `secrets/certificate_RBHU_SB_KONG_PROD.p12`
(override with `RBHU_ENV_FILE` / `RBHU_P12`).

## Local setup (Claude Desktop / Claude Code)

`claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "rbhu-ais": {
      "command": "/abs/path/to/bin/rbhu-connector",
      "env": {
        "RBHU_ENV_FILE": "/abs/path/to/secrets/.env",
        "RBHU_P12": "/abs/path/to/secrets/certificate_RBHU_SB_KONG_PROD.p12"
      }
    }
  }
}
```

Typical flow in chat:

1. "Create a consent for IBAN HU19120010080010059400100008, PSU 82742150."
   → `create_consent` returns an `authorize_url`.
2. Open the URL, complete SCA (sandbox login is PSU-ID only), approve.
   Copy the `code` **and** `state` from the redirect URL.
3. "Submit code <code> state <state> for consent <consent_id>."
   → `submit_authorization_code` (the state must match the one create_consent
   issued — CSRF protection).
4. "List my accounts / show savings balance / show transactions."

(`authorize_in_browser` automates steps 2–3 when the app's redirect URI points
at a local callback such as `http://127.0.0.1:8089/callback`.)

## Web setup (claude.ai custom connector)

```sh
RBHU_CONNECTOR_TOKEN=$(openssl rand -hex 32) \
RBHU_CONNECTOR_ADDR=127.0.0.1:8080 ./bin/rbhu-connector-http   # MCP at /mcp
```

The endpoint **requires a bearer token** (`RBHU_CONNECTOR_TOKEN`); the server
refuses to start without one and returns 401 for requests missing
`Authorization: Bearer <token>`. It binds to loopback by default. The
`authorize_in_browser` tool is **not** exposed over HTTP (browser SCA is local
only); use `create_consent` + `submit_authorization_code` instead.

Expose it over HTTPS (for local testing, a tunnel like `cloudflared` or
`ngrok`), then in **claude.ai → Settings → Connectors → Add custom connector**
enter the MCP URL, e.g. `https://<tunnel-host>/mcp`, and configure the same
bearer token. The read tools appear.

### Dev vs production

- **Dev (now):** the HTTP server uses one shared sandbox session — fine for a
  single tester. No connector-side OAuth.
- **Production (later):** each claude.ai user must get their own RBHU token.
  That means implementing the connector's own OAuth (claude.ai authorizes the
  user → the connector runs the RBHU consent + bridge SCA per user → stores a
  per-session token), and serving over real HTTPS with the production eIDAS
  QWAC certificate and production base URLs. The tool layer stays unchanged; the
  session/token wiring becomes per-user.
