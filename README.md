# rbhu

Go client for the **Raiffeisen Bank Zrt. (Hungary)** PSD2 Open Banking APIs,
compliant with the **Berlin Group NextGenPSD2 standard 1.3.2**.

Typed clients are generated from the official OpenAPI specifications
([`specs/`](specs/)) with [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen)
and live under [`psd2/`](psd2/). The root `rbhu` package wraps them with an
ergonomic layer: configuration, mutual-TLS, the OAuth bridge token flow,
Berlin Group request headers, and one-call helpers.

The default target is the RBHU **sandbox** (`RBHU_SB_KONG_PROD`). The full AIS
flow is verified end-to-end against the live sandbox.

## Install

```sh
go get github.com/s4mur4i/rbhu
```

## Quickstart

Put your credentials and certificate under `secrets/` (git-ignored):

```
secrets/.env                              # client_id, client_secret, redirect_url, eidas
secrets/certificate_RBHU_SB_KONG_PROD.p12 # sandbox client certificate (no password)
```

Create a consent and print the SCA authorization URL:

```go
client, err := rbhu.NewSandboxFromEnv("", "") // loads secrets/.env + secrets/*.p12
if err != nil {
	log.Fatal(err)
}

consent, err := client.CreateAISConsent(context.Background(), rbhu.AISConsentParams{
	IBANs: []string{"HU19120010080010059400100008"},
	PSUID: "82742150",
})
if err != nil {
	log.Fatal(err)
}
fmt.Println("authorize:", client.AuthorizeURL(rbhu.ScopeAISP, consent.ID, ""))
```

Complete SCA and read accounts in one call (local callback captures the code):

```go
_, err = client.CompleteAuthorization(ctx, rbhu.ScopeAISP, consent.ID,
	"127.0.0.1:8089", func(url string) { /* open url in a browser */ })
// token is now stored on the client
accs, _ := client.ListAccounts(ctx, consent.ID)
bal, _ := client.Balances(ctx, consent.ID, accs[0].ResourceId)
tx, _  := client.Transactions(ctx, consent.ID, accs[0].ResourceId, "booked")
```

See [`examples/ais`](examples/ais) (consent only) and [`examples/sca`](examples/sca)
(full flow) for runnable programs.

## API coverage

| Group | Helpers | Raw clients |
|---|---|---|
| AIS (accounts) | `CreateAISConsent`, `ListAccounts`, `Balances`, `Transactions` | `Accounts()`, `Consent()`, `ConsentAuth()` |
| PIS (payments) | — | `Payments()`, `BulkPayments()`, `CleanUpPayments()`, `SigningBaskets()` |
| Periodic payments | — | `Periodic()`, `PeriodicDomestic()` |
| CAF (funds) | — | `CAF()`, `CAFConsent()` |

The `*()` accessors return the typed generated clients (under `psd2/`),
pre-wired with the base URL, the (m)TLS HTTP client and the request editor.

## Authentication flow

1. **Create a consent** — `CreateAISConsent` (uses the client certificate; no
   token needed). Returns the `consentId`.
2. **SCA** — the PSU opens `AuthorizeURL(...)`, logs in (sandbox: PSU-ID only)
   and approves; the bank redirects back with a `?code=...`.
3. **Token** — exchange the code (`ExchangeToken`, or done for you by
   `CompleteAuthorization`); the token is stored on the client.
4. **Read** — `ListAccounts` / `Balances` / `Transactions`.

The client injects the headers the gateway requires on every call:
bearer `Authorization`, the marketplace `client_id` (key-auth),
`Accept: application/json`, a non-default `User-Agent` (the default Go one is
WAF-blocked), `X-Request-ID` and `Date`.

## Claude connector (read-only)

An MCP connector exposes the AIS read-only APIs to Claude, over stdio (local,
Claude Desktop / Claude Code) and Streamable HTTP (remote, claude.ai custom
connector). Tools: `create_consent`, `submit_authorization_code`,
`authorize_in_browser`, `list_accounts`, `get_balances`, `get_transactions`
(scope `AISP` only — no write operations). Build with `make connectors`; see
[docs/connector.md](docs/connector.md) for setup.

## Repository layout

```
*.go            rbhu package: client, config, oauth, errors, helpers, services
psd2/           generated typed clients, one package per OpenAPI spec
connector/      MCP connector for Claude (AIS read-only tools)
cmd/            connector binaries (stdio + Streamable HTTP)
specs/          OpenAPI specifications (source of truth for codegen)
scripts/        codegen driver + spec sanitizer
examples/       runnable examples (ais, sca)
docs/           API inventory, connector guide, notes
secrets/        credentials + certificate (git-ignored, not committed)
```

## Development

```sh
make tools      # install ./bin/oapi-codegen
make generate   # regenerate psd2/ from specs/  (also: go generate ./...)
make build
make test       # offline unit tests
make integration  # live sandbox tests (see below)
```

Regeneration sanitizes specs through `scripts/sanitize_spec.py` (strips
`example`/`examples` blocks that the strict OpenAPI loader rejects); the spec
files in `specs/` are left untouched.

### Integration tests

Excluded from the default build (`//go:build integration`); they hit the live
sandbox. Consent creation is fully automated and verified passing:

```sh
export RBHU_RUN_INTEGRATION=1
go test -tags integration -run TestIntegrationCreateConsent -v ./...
```

`TestIntegrationAuthorizedReads` needs an access token from a completed SCA
(`RBHU_ACCESS_TOKEN` + `RBHU_CONSENT_ID`).

## Security

Credentials live in `secrets/` and are git-ignored along with `*.p12`/`.env`.
Never commit real credentials or certificates.

## License

[MIT](LICENSE).
