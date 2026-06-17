package rbhu

import (
	"github.com/s4mur4i/rbhu/psd2/accounts"
	"github.com/s4mur4i/rbhu/psd2/bulkpayments"
	"github.com/s4mur4i/rbhu/psd2/caf"
	"github.com/s4mur4i/rbhu/psd2/cafconsent"
	"github.com/s4mur4i/rbhu/psd2/cleanuppayments"
	"github.com/s4mur4i/rbhu/psd2/consent"
	"github.com/s4mur4i/rbhu/psd2/consentauth"
	"github.com/s4mur4i/rbhu/psd2/payments"
	"github.com/s4mur4i/rbhu/psd2/periodic"
	"github.com/s4mur4i/rbhu/psd2/periodicdomestic"
	"github.com/s4mur4i/rbhu/psd2/signingbaskets"
)

// The accessors below return typed clients (generated from the OpenAPI specs)
// pre-wired with the environment base URL, the (m)TLS HTTP client and the
// Berlin Group request editor (bearer token, X-Request-ID, Date).

// Accounts returns the AIS Accounts API client.
func (c *Client) Accounts() (*accounts.ClientWithResponses, error) {
	return accounts.NewClientWithResponses(c.env.URL(ServiceAccounts),
		accounts.WithHTTPClient(c.httpClient), accounts.WithRequestEditorFn(c.requestEditor))
}

// Consent returns the AIS Consent API client.
func (c *Client) Consent() (*consent.ClientWithResponses, error) {
	return consent.NewClientWithResponses(c.env.URL(ServiceConsent),
		consent.WithHTTPClient(c.httpClient), consent.WithRequestEditorFn(c.requestEditor))
}

// ConsentAuth returns the AIS Consent Authorization API client.
func (c *Client) ConsentAuth() (*consentauth.ClientWithResponses, error) {
	return consentauth.NewClientWithResponses(c.env.URL(ServiceConsentAuth),
		consentauth.WithHTTPClient(c.httpClient), consentauth.WithRequestEditorFn(c.requestEditor))
}

// CAFConsent returns the BGS Consent CISP API client.
func (c *Client) CAFConsent() (*cafconsent.ClientWithResponses, error) {
	return cafconsent.NewClientWithResponses(c.env.URL(ServiceCAFConsent),
		cafconsent.WithHTTPClient(c.httpClient), cafconsent.WithRequestEditorFn(c.requestEditor))
}

// CAF returns the Confirmation of Funds API client.
func (c *Client) CAF() (*caf.ClientWithResponses, error) {
	return caf.NewClientWithResponses(c.env.URL(ServiceCAF),
		caf.WithHTTPClient(c.httpClient), caf.WithRequestEditorFn(c.requestEditor))
}

// BulkPayments returns the Bulk Payments API client.
func (c *Client) BulkPayments() (*bulkpayments.ClientWithResponses, error) {
	return bulkpayments.NewClientWithResponses(c.env.URL(ServiceBulkPayments),
		bulkpayments.WithHTTPClient(c.httpClient), bulkpayments.WithRequestEditorFn(c.requestEditor))
}

// CleanUpPayments returns the Clean-Up Payments API client.
func (c *Client) CleanUpPayments() (*cleanuppayments.ClientWithResponses, error) {
	return cleanuppayments.NewClientWithResponses(c.env.URL(ServiceCleanUpPayments),
		cleanuppayments.WithHTTPClient(c.httpClient), cleanuppayments.WithRequestEditorFn(c.requestEditor))
}

// Payments returns the Payment details API client.
func (c *Client) Payments() (*payments.ClientWithResponses, error) {
	return payments.NewClientWithResponses(c.env.URL(ServicePayments),
		payments.WithHTTPClient(c.httpClient), payments.WithRequestEditorFn(c.requestEditor))
}

// SigningBaskets returns the Signing Baskets API client.
func (c *Client) SigningBaskets() (*signingbaskets.ClientWithResponses, error) {
	return signingbaskets.NewClientWithResponses(c.env.URL(ServiceSigningBaskets),
		signingbaskets.WithHTTPClient(c.httpClient), signingbaskets.WithRequestEditorFn(c.requestEditor))
}

// PeriodicDomestic returns the Periodic Domestic Payments API client.
func (c *Client) PeriodicDomestic() (*periodicdomestic.ClientWithResponses, error) {
	return periodicdomestic.NewClientWithResponses(c.env.URL(ServicePeriodicDomestic),
		periodicdomestic.WithHTTPClient(c.httpClient), periodicdomestic.WithRequestEditorFn(c.requestEditor))
}

// Periodic returns the Periodic Payments API client.
func (c *Client) Periodic() (*periodic.ClientWithResponses, error) {
	return periodic.NewClientWithResponses(c.env.URL(ServicePeriodic),
		periodic.WithHTTPClient(c.httpClient), periodic.WithRequestEditorFn(c.requestEditor))
}
