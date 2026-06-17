# CLAUDE.md

Go client + Claude connector for the **Raiffeisen Hungary (RBHU) PSD2** Open
Banking APIs (Berlin Group NextGenPSD2 1.3.2). Default target is the RBHU
sandbox.

## Layout

- root `*.go` — `rbhu` package: client, config, OAuth, helpers, errors.
- `psd2/` — typed clients generated from `specs/` (do not hand-edit `*.gen.go`).
- `connector/` + `cmd/` — MCP connector (AIS read-only) for Claude.
- `secrets/` — credentials + certificate (git-ignored; never commit).

## Build / test

```sh
make build          # compile everything
make test           # offline unit tests
make connectors     # build bin/rbhu-connector (stdio) + bin/rbhu-connector-http
make generate       # regenerate psd2/ from specs/ (needs `make tools`)
```

## Install the connector for Claude (local)

The connector is **AIS read-only** (scope AISP) — no payment/write tools.

1. Provide credentials: copy `.env.example` to `secrets/.env`, fill it in, and
   place the sandbox certificate at
   `secrets/certificate_RBHU_SB_KONG_PROD.p12` (the sandbox test cert has no
   password).
2. Build + register, one command:
   ```sh
   make install
   ```
   Then restart Claude Code (or `/mcp` → reconnect).

   Alternatively, inside this repo a `.mcp.json` is bundled: just
   `make connectors` and approve the project MCP server when Claude Code
   prompts — no command needed.

## Use it in chat

1. "Create an AIS consent for IBAN HU19120010080010059400100008, PSU 82742150."
2. Open the returned `authorize_url`, log in (sandbox: PSU-ID only), approve;
   copy `code` and `state` from the redirect URL.
3. "Submit code <code> state <state> for consent <consent_id>."
4. "List my accounts / show balances / show transactions."

## Web (claude.ai) connector

```sh
make connector-dev   # HTTP server + bearer token + public tunnel
```
Add the printed `<url>/mcp` and bearer token in claude.ai → Connectors.

## Sandbox test data

PSU `82742150`, IBAN `HU19120010080010059400100008` (HUF) has accounts,
balances and transactions. More in `docs/sandbox/` (git-ignored, local only).

## Conventions

- Never commit anything under `secrets/` or any `.env`/`*.p12`.
- The RBHU gateway needs `Accept: application/json`, the `client_id` header,
  and a non-default `User-Agent` — the client sets these automatically.
- Some live payloads are richer than the spec; the connector reads
  balances/transactions raw (`Client.BalancesRaw`/`TransactionsRaw`).
