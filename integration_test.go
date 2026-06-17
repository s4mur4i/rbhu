//go:build integration

// Integration tests run against the live RBHU sandbox. They are excluded from
// the default build and require:
//
//	RBHU_RUN_INTEGRATION=1
//	a .env file with client_id/client_secret/redirect_url   (RBHU_ENV_FILE, default ./.env)
//	a client certificate                                    (RBHU_P12, default ./certificate_RBHU_SB_KONG_PROD.p12)
//	RBHU_P12_PASSWORD                                        (the .p12 password, may be empty)
//	RBHU_TEST_IBAN                                           (a sandbox test PSU IBAN)
//
// Run with:  go test -tags integration -run TestIntegration -v ./...
package rbhu

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/s4mur4i/rbhu/psd2/accounts"
	"github.com/s4mur4i/rbhu/psd2/consent"
)

func integrationClient(t *testing.T) *Client {
	t.Helper()
	if os.Getenv("RBHU_RUN_INTEGRATION") != "1" {
		t.Skip("set RBHU_RUN_INTEGRATION=1 to run live sandbox tests")
	}
	envFile := getenvDefault("RBHU_ENV_FILE", "secrets/.env")
	cfg, err := ConfigFromDotEnv(envFile)
	if err != nil {
		t.Fatalf("load %s: %v", envFile, err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	p12 := getenvDefault("RBHU_P12", "secrets/certificate_RBHU_SB_KONG_PROD.p12")
	cert, err := LoadCertificate(p12, os.Getenv("RBHU_P12_PASSWORD"))
	if err != nil {
		t.Fatalf("load cert %s: %v", p12, err)
	}
	cfg.Certificate = cert
	return New(cfg)
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// Documented sandbox test data (see ./sandbox). PSU 82742150 / this IBAN has
// account, balance and transaction data available.
const (
	defaultTestPSUID = "82742150"
	defaultTestIBAN  = "HU19120010080010059400100008"
)

func testPSU() (psuID, iban string) {
	psuID = getenvDefault("RBHU_TEST_PSU_ID", defaultTestPSUID)
	iban = getenvDefault("RBHU_TEST_IBAN", defaultTestIBAN)
	return
}

// TestIntegrationCreateConsent creates an AIS consent against the live sandbox
// and prints the consent id and the bridge authorization URL for SCA. The
// consent-creation step uses the client certificate (mutual TLS) and does not
// require a bearer token.
func TestIntegrationCreateConsent(t *testing.T) {
	c := integrationClient(t)
	psuID, iban := testPSU()

	api, err := c.Consent()
	if err != nil {
		t.Fatal(err)
	}

	acct := []consent.XS2ABerlinConsentResources{{Iban: &iban}}
	body := consent.NewConsentJSONRequestBody{
		Access: consent.XS2ABerlinConsentAccountAccess{
			Accounts:     &acct,
			Balances:     &acct,
			Transactions: &acct,
		},
		RecurringIndicator:       true,
		CombinedServiceIndicator: false,
		FrequencyPerDay:          4,
		ValidUntil:               openapi_types.Date{Time: time.Now().AddDate(0, 0, 90)},
	}

	resp, err := api.NewConsentWithResponse(context.Background(), &consent.NewConsentParams{
		ClientId:   c.cfg.ClientID,
		XRequestID: newRequestID(),
		PSUID:      &psuID,
	}, body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.JSON200 == nil || resp.JSON200.ConsentId == nil {
		t.Fatalf("create consent: status=%d body=%s", resp.StatusCode(), string(resp.Body))
	}
	consentID := *resp.JSON200.ConsentId
	t.Logf("consentId = %s", consentID)
	t.Logf("consentStatus = %v", resp.JSON200.ConsentStatus)
	t.Logf("authorize = %s", c.AuthorizeURL(ScopeAISP, consentID, "test-state"))
}

// TestIntegrationAuthorizedReads exercises the calls that require an access
// token obtained out-of-band via the bridge SCA flow (the PSU logs in and
// authorizes the consent). Supply the token and the consent id it was issued
// for via env:
//
//	RBHU_ACCESS_TOKEN, RBHU_CONSENT_ID
//
// It reads the consent status, the consent details and the account list.
func TestIntegrationAuthorizedReads(t *testing.T) {
	c := integrationClient(t)
	token := os.Getenv("RBHU_ACCESS_TOKEN")
	consentID := os.Getenv("RBHU_CONSENT_ID")
	if token == "" || consentID == "" {
		t.Skip("set RBHU_ACCESS_TOKEN and RBHU_CONSENT_ID (from a completed SCA) to run authorized reads")
	}
	c.SetToken(&Token{AccessToken: token})

	consentAPI, err := c.Consent()
	if err != nil {
		t.Fatal(err)
	}

	statusResp, err := consentAPI.GetConsentStatusWithResponse(context.Background(), consentID,
		&consent.GetConsentStatusParams{XRequestID: newRequestID()})
	if err != nil {
		t.Fatal(err)
	}
	if statusResp.JSON200 == nil {
		t.Fatalf("get status: status=%d body=%s", statusResp.StatusCode(), string(statusResp.Body))
	}
	t.Logf("consentStatus = %v", statusResp.JSON200.ConsentStatus)

	getResp, err := consentAPI.GetConsentWithResponse(context.Background(), consentID,
		&consent.GetConsentParams{ClientId: c.cfg.ClientID, XRequestID: newRequestID()})
	if err != nil {
		t.Fatal(err)
	}
	if getResp.JSON200 == nil {
		t.Fatalf("get consent: status=%d body=%s", getResp.StatusCode(), string(getResp.Body))
	}
	t.Logf("consent retrieved, status=%v", getResp.JSON200.ConsentStatus)

	accAPI, err := c.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	accResp, err := accAPI.GetAccountListWithResponse(context.Background(), &accounts.GetAccountListParams{
		XRequestID: openapi_types.UUID(uuid.New()),
		ConsentID:  consentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if accResp.JSON200 == nil {
		t.Fatalf("get accounts: status=%d body=%s", accResp.StatusCode(), string(accResp.Body))
	}
	t.Logf("accounts response: %s", string(accResp.Body))
}
