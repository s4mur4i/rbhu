// Package connector exposes the RBHU AIS (read-only) APIs as MCP tools for use
// as a Claude connector. The same tool set is served over stdio (local, e.g.
// Claude Desktop / Claude Code) and over Streamable HTTP (remote, e.g. a
// claude.ai custom connector).
//
// It is strictly read-only: it can create an account-information consent,
// complete SCA, and read accounts, balances and transactions. No payment or
// other write operations are exposed.
package connector

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/s4mur4i/rbhu"
	"github.com/s4mur4i/rbhu/psd2/accounts"
)

// Version is the connector's reported version.
const Version = "0.1.0"

// Option configures the connector.
type Option func(*config)

type config struct {
	browserAuth bool
}

// WithBrowserAuth registers the authorize_in_browser tool, which opens a local
// browser and runs a local callback server. Enable it ONLY for local (stdio)
// transports — never on a remote HTTP server, where it would let remote callers
// trigger server-side browser launches and local port binds.
func WithBrowserAuth() Option { return func(c *config) { c.browserAuth = true } }

// conn holds the per-connector state shared across tool handlers.
type conn struct {
	c      *rbhu.Client
	mu     sync.Mutex
	states map[string]string // consentID -> issued OAuth state (CSRF token)
}

// New builds an MCP server exposing the AIS read-only tools, backed by the
// given RBHU client.
func New(c *rbhu.Client, opts ...Option) *mcp.Server {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	cn := &conn{c: c, states: make(map[string]string)}

	s := mcp.NewServer(&mcp.Implementation{
		Name:    "rbhu-ais",
		Title:   "Raiffeisen Hungary (AIS, read-only)",
		Version: Version,
	}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_consent",
		Title:       "Create account-information consent",
		Description: "Create a read-only AIS consent for one or more IBANs and return the consent id, an SCA authorize URL and the state value to echo back. Does not move money.",
	}, cn.createConsent)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "submit_authorization_code",
		Title:       "Submit SCA authorization code",
		Description: "Exchange the authorization code from the SCA redirect for an access token. Provide the consent_id, the code, and the state, both taken from the redirect URL.",
	}, cn.submitCode)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_accounts",
		Title:       "List accounts",
		Description: "List the accounts (current and savings) covered by an authorized consent. Requires a prior authorization.",
	}, cn.listAccounts)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_balances",
		Title:       "Get account balances",
		Description: "Get the balances of one account under an authorized consent.",
	}, cn.getBalances)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_transactions",
		Title:       "Get account transactions",
		Description: "Get the transactions of one account under an authorized consent. bookingStatus is booked, pending or both (default booked).",
	}, cn.getTransactions)

	if cfg.browserAuth {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "authorize_in_browser",
			Title:       "Authorize via local browser (local setup only)",
			Description: "Open the SCA URL in a local browser and automatically capture the authorization code on a loopback callback server. Local setup only.",
		}, cn.authorizeBrowser)
	}

	return s
}

func newState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---- create_consent ----

type CreateConsentIn struct {
	IBANs []string `json:"ibans" jsonschema:"the account IBANs the consent should cover"`
	PSUID string   `json:"psu_id" jsonschema:"the customer's PSU identifier"`
}

type CreateConsentOut struct {
	ConsentID    string `json:"consent_id"`
	Status       string `json:"status"`
	AuthorizeURL string `json:"authorize_url"`
	State        string `json:"state"`
}

func (cn *conn) createConsent(ctx context.Context, _ *mcp.CallToolRequest, in CreateConsentIn) (*mcp.CallToolResult, CreateConsentOut, error) {
	consent, err := cn.c.CreateAISConsent(ctx, rbhu.AISConsentParams{
		IBANs: in.IBANs, PSUID: in.PSUID, Recurring: true,
	})
	if err != nil {
		return errResult(err), CreateConsentOut{}, nil
	}
	state := newState()
	cn.mu.Lock()
	cn.states[consent.ID] = state
	cn.mu.Unlock()

	return nil, CreateConsentOut{
		ConsentID:    consent.ID,
		Status:       consent.Status,
		AuthorizeURL: cn.c.AuthorizeURL(rbhu.ScopeAISP, consent.ID, state),
		State:        state,
	}, nil
}

// ---- submit_authorization_code ----

type SubmitCodeIn struct {
	ConsentID string `json:"consent_id" jsonschema:"the consent id from create_consent"`
	Code      string `json:"code" jsonschema:"the authorization code from the redirect URL after SCA"`
	State     string `json:"state" jsonschema:"the state value from the redirect URL; must match the one issued by create_consent"`
}

type AuthOut struct {
	Status    string `json:"status"`
	ExpiresIn int    `json:"expires_in_seconds"`
}

func (cn *conn) submitCode(ctx context.Context, _ *mcp.CallToolRequest, in SubmitCodeIn) (*mcp.CallToolResult, AuthOut, error) {
	cn.mu.Lock()
	want, ok := cn.states[in.ConsentID]
	cn.mu.Unlock()
	if !ok || in.State == "" || subtle.ConstantTimeCompare([]byte(want), []byte(in.State)) != 1 {
		return errResult(fmt.Errorf("state mismatch or unknown consent_id (possible CSRF); call create_consent first and echo back its state")), AuthOut{}, nil
	}

	tok, err := cn.c.ExchangeToken(ctx, rbhu.ScopeAISP, in.Code)
	if err != nil {
		return errResult(err), AuthOut{}, nil
	}
	cn.mu.Lock()
	delete(cn.states, in.ConsentID)
	cn.mu.Unlock()
	return nil, AuthOut{Status: "authorized", ExpiresIn: tok.ExpiresIn}, nil
}

// ---- authorize_in_browser (local only) ----

type AuthorizeBrowserIn struct {
	ConsentID    string `json:"consent_id"`
	CallbackAddr string `json:"callback_addr" jsonschema:"loopback host:port the redirect is captured on (default 127.0.0.1:8089)"`
}

func (cn *conn) authorizeBrowser(ctx context.Context, _ *mcp.CallToolRequest, in AuthorizeBrowserIn) (*mcp.CallToolResult, AuthOut, error) {
	addr := in.CallbackAddr
	if addr == "" {
		addr = "127.0.0.1:8089"
	}
	if err := requireLoopback(addr); err != nil {
		return errResult(err), AuthOut{}, nil
	}
	tok, err := cn.c.CompleteAuthorization(ctx, rbhu.ScopeAISP, in.ConsentID, addr, openBrowser)
	if err != nil {
		return errResult(err), AuthOut{}, nil
	}
	return nil, AuthOut{Status: "authorized", ExpiresIn: tok.ExpiresIn}, nil
}

func requireLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid callback_addr %q: %w", addr, err)
	}
	switch host {
	case "127.0.0.1", "::1", "localhost", "":
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("callback_addr must be a loopback address, got %q", host)
}

// ---- list_accounts ----

type ConsentIn struct {
	ConsentID string `json:"consent_id"`
}

type AccountSummary struct {
	ResourceID      string `json:"resource_id"`
	IBAN            string `json:"iban,omitempty"`
	Currency        string `json:"currency"`
	Name            string `json:"name,omitempty"`
	OwnerName       string `json:"owner_name,omitempty"`
	Product         string `json:"product,omitempty"`
	CashAccountType string `json:"cash_account_type,omitempty"` // CACC=current, SVGS=savings
}

type ListAccountsOut struct {
	Accounts []AccountSummary `json:"accounts"`
}

func (cn *conn) listAccounts(ctx context.Context, _ *mcp.CallToolRequest, in ConsentIn) (*mcp.CallToolResult, ListAccountsOut, error) {
	accs, err := cn.c.ListAccounts(ctx, in.ConsentID)
	if err != nil {
		return errResult(err), ListAccountsOut{}, nil
	}
	out := ListAccountsOut{Accounts: make([]AccountSummary, 0, len(accs))}
	for _, a := range accs {
		out.Accounts = append(out.Accounts, AccountSummary{
			ResourceID:      a.ResourceId,
			IBAN:            deref(a.Iban),
			Currency:        a.Currency,
			Name:            deref(a.Name),
			OwnerName:       derefOwner(a.OwnerName),
			Product:         deref(a.Product),
			CashAccountType: cashType(a.CashAccountType),
		})
	}
	return nil, out, nil
}

// ---- get_balances ----

type AccountIn struct {
	ConsentID string `json:"consent_id"`
	AccountID string `json:"account_id" jsonschema:"the account resource_id from list_accounts"`
}

type RawOut struct {
	AccountID string `json:"account_id"`
	// Data is the raw API payload. Typed as any so the generated MCP output
	// schema is permissive (the live payloads are richer than the spec).
	Data any `json:"data"`
}

func (cn *conn) getBalances(ctx context.Context, _ *mcp.CallToolRequest, in AccountIn) (*mcp.CallToolResult, RawOut, error) {
	raw, err := cn.c.BalancesRaw(ctx, in.ConsentID, in.AccountID)
	if err != nil {
		return errResult(err), RawOut{}, nil
	}
	return nil, RawOut{AccountID: in.AccountID, Data: jsonToAny(raw)}, nil
}

// ---- get_transactions ----

type TransactionsIn struct {
	ConsentID     string `json:"consent_id"`
	AccountID     string `json:"account_id" jsonschema:"the account resource_id from list_accounts"`
	BookingStatus string `json:"booking_status" jsonschema:"booked, pending or both (default booked)"`
}

func (cn *conn) getTransactions(ctx context.Context, _ *mcp.CallToolRequest, in TransactionsIn) (*mcp.CallToolResult, RawOut, error) {
	raw, err := cn.c.TransactionsRaw(ctx, in.ConsentID, in.AccountID, in.BookingStatus)
	if err != nil {
		return errResult(err), RawOut{}, nil
	}
	return nil, RawOut{AccountID: in.AccountID, Data: jsonToAny(raw)}, nil
}

// jsonToAny decodes raw JSON into an any value for permissive MCP output.
func jsonToAny(raw []byte) any {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

// ---- helpers ----

func errResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefOwner(o *accounts.OwnerName) string {
	if o == nil {
		return ""
	}
	return string(*o)
}

func cashType(t *accounts.XS2ABerlinAccountCashAccountType) string {
	if t == nil {
		return ""
	}
	return string(*t)
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	_ = exec.Command(cmd, append(args, url)...).Start()
}
