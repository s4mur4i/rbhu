package rbhu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/s4mur4i/rbhu/psd2/accounts"
	"github.com/s4mur4i/rbhu/psd2/consent"
)

// NewSandboxFromEnv is a one-call constructor: it loads credentials from a
// .env file and a client certificate from a PKCS#12 bundle (no password, as
// the sandbox test certificate is unprotected) and returns a Client targeting
// the sandbox. Empty paths default to ".env" and
// "certificate_RBHU_SB_KONG_PROD.p12".
func NewSandboxFromEnv(envPath, p12Path string) (*Client, error) {
	if envPath == "" {
		envPath = "secrets/.env"
	}
	if p12Path == "" {
		p12Path = "secrets/certificate_RBHU_SB_KONG_PROD.p12"
	}
	cfg, err := ConfigFromDotEnv(envPath)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cert, err := LoadCertificate(p12Path, "")
	if err != nil {
		return nil, err
	}
	cfg.Certificate = cert
	return New(cfg, WithEnvironment(Sandbox)), nil
}

func newUUID() openapi_types.UUID { return openapi_types.UUID(uuid.New()) }

// AISConsentParams describes an account-information consent to create.
type AISConsentParams struct {
	// IBANs are the dedicated accounts the consent covers. RBHU requires at
	// least one (consent without dedicated accounts is not supported).
	IBANs []string
	// PSUID is the PSU identifier (uppercased for corporate IDs).
	PSUID string
	// ValidUntil defaults to 90 days from now if zero.
	ValidUntil time.Time
	// FrequencyPerDay defaults to 4 if zero.
	FrequencyPerDay int
	// Recurring marks the consent as recurring (default true via CreateAISConsent).
	Recurring bool
}

// Consent is the result of creating a consent.
type Consent struct {
	ID          string
	Status      string
	SCARedirect string // bank-provided SCA redirect link, if any
}

// CreateAISConsent creates an AIS consent for the given accounts and returns
// its id, status and SCA redirect link. Uses the client certificate; no bearer
// token is required for this step.
func (c *Client) CreateAISConsent(ctx context.Context, p AISConsentParams) (*Consent, error) {
	if len(p.IBANs) == 0 {
		return nil, fmt.Errorf("rbhu: CreateAISConsent requires at least one IBAN")
	}
	validUntil := p.ValidUntil
	if validUntil.IsZero() {
		validUntil = time.Now().AddDate(0, 0, 90)
	}
	freq := p.FrequencyPerDay
	if freq == 0 {
		freq = 4
	}

	res := make([]consent.XS2ABerlinConsentResources, len(p.IBANs))
	for i, iban := range p.IBANs {
		iban := iban
		res[i] = consent.XS2ABerlinConsentResources{Iban: &iban}
	}

	api, err := c.Consent()
	if err != nil {
		return nil, err
	}
	params := &consent.NewConsentParams{ClientId: c.cfg.ClientID, XRequestID: newRequestID()}
	if p.PSUID != "" {
		params.PSUID = &p.PSUID
	}
	resp, err := api.NewConsentWithResponse(ctx, params, consent.NewConsentJSONRequestBody{
		Access: consent.XS2ABerlinConsentAccountAccess{
			Accounts: &res, Balances: &res, Transactions: &res,
		},
		RecurringIndicator:       p.Recurring,
		CombinedServiceIndicator: false,
		FrequencyPerDay:          freq,
		ValidUntil:               openapi_types.Date{Time: validUntil},
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil || resp.JSON200.ConsentId == nil {
		return nil, errorFromResponse(resp.StatusCode(), resp.HTTPResponse, resp.Body)
	}
	out := &Consent{
		ID:          *resp.JSON200.ConsentId,
		Status:      string(resp.JSON200.ConsentStatus),
		SCARedirect: resp.JSON200.UnderscoreLinks.ScaRedirect.Href,
	}
	return out, nil
}

// ListAccounts returns the accounts covered by an authorized consent. Requires
// an access token (see CompleteAuthorization).
func (c *Client) ListAccounts(ctx context.Context, consentID string) ([]accounts.XS2ABerlinAccount, error) {
	api, err := c.Accounts()
	if err != nil {
		return nil, err
	}
	resp, err := api.GetAccountListWithResponse(ctx, &accounts.GetAccountListParams{
		XRequestID: newUUID(), ConsentID: consentID,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, errorFromResponse(resp.StatusCode(), resp.HTTPResponse, resp.Body)
	}
	return resp.JSON200.Accounts, nil
}

// Balances returns the balances of one account under an authorized consent.
func (c *Client) Balances(ctx context.Context, consentID, accountID string) (*accounts.XS2ABerlinBalanceResponse, error) {
	api, err := c.Accounts()
	if err != nil {
		return nil, err
	}
	resp, err := api.GetBalancesWithResponse(ctx, accountID, &accounts.GetBalancesParams{
		XRequestID: newUUID(), ConsentID: consentID,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, errorFromResponse(resp.StatusCode(), resp.HTTPResponse, resp.Body)
	}
	return resp.JSON200, nil
}

// Transactions returns the transactions of one account under an authorized
// consent. bookingStatus is one of "booked", "pending" or "both".
func (c *Client) Transactions(ctx context.Context, consentID, accountID, bookingStatus string) (*accounts.XS2ABerlinTransactionsListResponse, error) {
	if bookingStatus == "" {
		bookingStatus = "booked"
	}
	api, err := c.Accounts()
	if err != nil {
		return nil, err
	}
	resp, err := api.GetTransactionListWithResponse(ctx, accountID, &accounts.GetTransactionListParams{
		XRequestID:    newUUID(),
		ConsentID:     consentID,
		BookingStatus: accounts.GetTransactionListParamsBookingStatus(bookingStatus),
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, errorFromResponse(resp.StatusCode(), resp.HTTPResponse, resp.Body)
	}
	return resp.JSON200, nil
}

// accountsRawClient builds the low-level (non-typed) accounts client, wired
// like Accounts() but returning raw *http.Response. Used to read endpoints
// whose live payloads do not match the OpenAPI spec strictly enough for the
// generated typed decoder.
func (c *Client) accountsRawClient() (*accounts.Client, error) {
	return accounts.NewClient(c.env.URL(ServiceAccounts),
		accounts.WithHTTPClient(c.httpClient), accounts.WithRequestEditorFn(c.requestEditor))
}

// BalancesRaw returns the raw balances JSON for an account (no strict typing).
func (c *Client) BalancesRaw(ctx context.Context, consentID, accountID string) (json.RawMessage, error) {
	lc, err := c.accountsRawClient()
	if err != nil {
		return nil, err
	}
	resp, err := lc.GetBalances(ctx, accountID, &accounts.GetBalancesParams{
		XRequestID: newUUID(), ConsentID: consentID,
	})
	if err != nil {
		return nil, err
	}
	return readRawBody(resp)
}

// TransactionsRaw returns the raw transactions JSON for an account (no strict
// typing). bookingStatus is booked, pending or both (default booked).
func (c *Client) TransactionsRaw(ctx context.Context, consentID, accountID, bookingStatus string) (json.RawMessage, error) {
	if bookingStatus == "" {
		bookingStatus = "booked"
	}
	lc, err := c.accountsRawClient()
	if err != nil {
		return nil, err
	}
	resp, err := lc.GetTransactionList(ctx, accountID, &accounts.GetTransactionListParams{
		XRequestID:    newUUID(),
		ConsentID:     consentID,
		BookingStatus: accounts.GetTransactionListParamsBookingStatus(bookingStatus),
	})
	if err != nil {
		return nil, err
	}
	return readRawBody(resp)
}

func readRawBody(resp *http.Response) (json.RawMessage, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, resp.Header.Get("X-Request-ID"), body)
	}
	return body, nil
}

// errorFromResponse builds an APIError from a generated response's parts.
func errorFromResponse(status int, httpResp *http.Response, body []byte) error {
	var reqID string
	if httpResp != nil {
		reqID = httpResp.Header.Get("X-Request-ID")
	}
	return parseAPIError(status, reqID, body)
}

// AuthResult is the outcome of the OAuth redirect after SCA.
type AuthResult struct {
	Code  string
	State string
}

// CodeListener is a tiny local HTTP server that captures the OAuth
// authorization code from the redirect after the PSU completes SCA. Use it
// when the application's redirect URI points at a local address.
type CodeListener struct {
	ln  net.Listener
	srv *http.Server
	ch  chan AuthResult
	err chan error
}

// ListenForCode starts a local server on addr (e.g. "127.0.0.1:0") that
// captures the authorization code on any request carrying a "code" query
// parameter. Call Wait to block for the code, then Close.
func (c *Client) ListenForCode(addr string) (*CodeListener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	cl := &CodeListener{
		ln:  ln,
		ch:  make(chan AuthResult, 1),
		err: make(chan error, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if code := q.Get("code"); code != "" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html><body>Authorization complete. You may close this window.</body></html>"))
			select {
			case cl.ch <- AuthResult{Code: code, State: q.Get("state")}:
			default:
			}
			return
		}
		if e := q.Get("error"); e != "" {
			http.Error(w, e, http.StatusBadRequest)
			select {
			case cl.err <- fmt.Errorf("rbhu: authorization error: %s: %s", e, q.Get("error_description")):
			default:
			}
			return
		}
		http.Error(w, "waiting for authorization code", http.StatusBadRequest)
	})
	cl.srv = &http.Server{Handler: mux}
	go func() { _ = cl.srv.Serve(ln) }()
	return cl, nil
}

// Addr returns the address the listener is bound to (host:port).
func (cl *CodeListener) Addr() string { return cl.ln.Addr().String() }

// Wait blocks until the authorization code is received, the context is done,
// or an error redirect arrives.
func (cl *CodeListener) Wait(ctx context.Context) (*AuthResult, error) {
	select {
	case r := <-cl.ch:
		return &r, nil
	case err := <-cl.err:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close shuts the listener down.
func (cl *CodeListener) Close() error { return cl.srv.Close() }

// CompleteAuthorization runs the full redirect leg: it builds the bridge
// authorization URL, hands it to open (e.g. to print or launch a browser),
// captures the returned code on a local listener bound to listenAddr, exchanges
// it for an access token and stores the token on the client.
//
// The application's redirect URI must point at listenAddr for the capture to
// work (register a local redirect URI in the marketplace, e.g.
// http://127.0.0.1:8089/callback).
func (c *Client) CompleteAuthorization(ctx context.Context, scope Scope, consentID, listenAddr string, open func(authURL string)) (*Token, error) {
	cl, err := c.ListenForCode(listenAddr)
	if err != nil {
		return nil, err
	}
	defer cl.Close()

	state := uuid.NewString()
	authURL := c.AuthorizeURL(scope, consentID, state)
	if open != nil {
		open(authURL)
	}

	res, err := cl.Wait(ctx)
	if err != nil {
		return nil, err
	}
	if res.State != state {
		return nil, fmt.Errorf("rbhu: state mismatch on authorization callback (possible CSRF)")
	}
	return c.ExchangeToken(ctx, scope, res.Code)
}
