# RBHU PSD2 API Inventory (Raiffeisen Bank Zrt., Hungary)

Extracted from the RBI API Marketplace public catalog
(`https://api.rbinternational.com/catalog/{providers,categories,bundles}`).

## Provider

- name: **Raiffeisen Bank Zrt.**
- key: `raiffeisenbank-zrt`
- id: `207a3afe-b472-11ea-b3de-0242ac130004`
- country: Hungary (HU)
- contact: `psd2@raiffeisen.hu`
- standard: Berlin Group NextGenPSD2 **1.3.2**

### Environments

| envName | type | auth | clientCert |
|---|---|---|---|
| `RBHU_SB_KONG_PROD` | SANDBOX | OAUTH2 | yes |
| `RBHU_KONG_PROD` | PRODUCTION | OAUTH2 | no |

## API bundles (5)

Spec paths are relative keys served by an auth-gated endpoint
(`/catalog/...` → HTTP 403 "Missing Authentication Token" without a bearer token).
`prod02` = sandbox, `prod01` = production.

| Bundle | API name | version | sandbox spec path | endpoint |
|---|---|---|---|---|
| Accounts API Bundle (AIS) | `psd2-accounts-api-132` | 1.3.2.2 | `rbhu/prod02/psd2-accounts-api-1.3.2.yaml` | MTLS_EP |
| Payments API Bundle (PIS) | `psd2-payments-api-132` | 1.3.2 | `rbhu/prod02/psd2-payments-api-1.3.2.yaml` | MTLS_EP |
| Periodic Payments API Bundle | `psd2-periodic-payments-api-132` | 1.3.2 | `rbhu/prod02/psd2-periodic-payments-api-1.3.2.yaml` | MTLS_EP |
| Confirmation of funds API Bundle (CAF) | `psd2-bgs-consent-cisp-api-200` | 2.0.0.0 | `rbhu/prod02/psd2-bgs-consent-cisp-api-2.0.0.yaml` | MTLS_EP |
| OAuth2 API Bundle | `psd2-bridge-oauth2-api` | 1.3.2.1 | `rbhu/prod02/psd2-rbhu-bridge-api.yaml` | TLS_EP |

### Notes

- AIS basepath changed in v1.3.2.1.
- Corporate internet banking PSU-IDs may contain `[A-Z]`; these MUST be
  uppercased when generating consents.
- Two endpoint types: `MTLS_EP` (mutual-TLS, data APIs) and `TLS_EP`
  (plain TLS, OAuth bridge).

## Live sandbox findings (verified)

- Sandbox hosts: data `hu-api-sandbox.raiffeisen.hu`, bridge
  `hu-bridge-sandbox.raiffeisen.hu`.
- The test certificate (`certificate_RBHU_SB_KONG_PROD.p12`) has **no
  password**; CN `www.rbi-test-tpp-sandbox.at`, issuer `Fina Demo CA 2020`.
- The gateway requires `Accept: application/json` (else HTTP 406
  `REQUESTED_FORMATS_INVALID`), the marketplace `client_id` header on every call
  (else HTTP 401 `No API key found in request`), and blocks the default
  `Go-http-client` User-Agent at the WAF (HTTP 403). The client sets all three
  automatically.
- SCA login is PSU-ID only (no password) at the Merlin sandbox login page;
  approving the consent redirects to `…/swagger-ui-oauth2-redirect?code=…`.
- Full flow verified end-to-end live: create consent (cert) → browser SCA →
  authorization code → token exchange → account list returns the expected
  sandbox account data (PSU 82742150, IBAN HU19120010080010059400100008, HUF).
- `POST /consents` works with the client certificate alone (no token) and
  returns `consentStatus: received` + an `scaRedirect` link.
- Consent status/details and account/balance/transaction reads require a bearer
  token obtained via the bridge SCA flow (interactive PSU login at
  `prod02-psd2-sandbox-web.openapi.merlinplatform.cloud/login`).
- Best full-data test account: PSU `82742150`, IBAN
  `HU19120010080010059400100008` (HUF; account + balance + transactions).

## What is still needed

The 5 OpenAPI YAML files above (exact paths, request/response schemas,
required Berlin Group headers per operation) and the sandbox test-data
values (test PSU credentials, IBANs, OAuth client). The catalog spec
download requires an authenticated bearer token from a logged-in session.
