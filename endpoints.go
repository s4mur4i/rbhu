package rbhu

import "strings"

// Environment identifies an RBHU PSD2 deployment.
type Environment struct {
	// APIBase is the root of the data APIs (mutual-TLS endpoint).
	APIBase string
	// BridgeBase is the root of the OAuth bridge (plain-TLS endpoint).
	BridgeBase string
}

// Sandbox is the RBHU PSD2 sandbox (RBHU_SB_KONG_PROD, prod02).
var Sandbox = Environment{
	APIBase:    "https://hu-api-sandbox.raiffeisen.hu/rbhu/prod02",
	BridgeBase: "https://hu-bridge-sandbox.raiffeisen.hu/rbhu/prod02/psd2-rbhu-bridge-api",
}

// Service identifies one of the generated API clients and its base path
// relative to Environment.APIBase.
type Service string

const (
	ServiceAccounts         Service = "psd2-accounts-api-1.3.2-rbhu/v1"
	ServiceConsent          Service = "psd2-bgs-consent-api-1.3.2-rbhu/v1"
	ServiceConsentAuth      Service = "psd2-bgs-authorisation-api/v1"
	ServiceCAFConsent       Service = "psd2-bgs-consent-cisp-api-2.0.0/v2"
	ServiceCAF              Service = "psd2-cards-api-1.3.2/v1"
	ServiceBulkPayments     Service = "psd2-bulk-payments-api-1.3.2/v1"
	ServiceCleanUpPayments  Service = "psd2-payments-cleanup-api-1.3.2/v1"
	ServicePayments         Service = "psd2-payments-api-1.3.2/v1"
	ServiceSigningBaskets   Service = "psd2-signing-baskets-api-1.3.2/v1"
	ServicePeriodicDomestic Service = "psd2-periodic-payments-api-1.3.2/v1"
	ServicePeriodic         Service = "psd2-periodic-payments-api-1.3.2/v1"
)

// URL returns the full base URL for a service in this environment, with a
// trailing slash so generated clients resolve operation paths correctly.
func (e Environment) URL(s Service) string {
	return strings.TrimRight(e.APIBase, "/") + "/" + string(s) + "/"
}

// TokenURL returns the OAuth2 token endpoint for the given scope
// (e.g. "aisp", "pisp", "cisp").
func (e Environment) TokenURL(scope string) string {
	return strings.TrimRight(e.APIBase, "/") + "/" + strings.ToLower(scope) + "/oauth2/token"
}

// AuthorizeURL returns the OAuth bridge authorize endpoint.
func (e Environment) AuthorizeURL() string {
	return strings.TrimRight(e.BridgeBase, "/") + "/bridge/authorize"
}
