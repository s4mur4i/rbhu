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
	"encoding/json"
	"os/exec"
	"runtime"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/s4mur4i/rbhu"
	"github.com/s4mur4i/rbhu/psd2/accounts"
)

// Version is the connector's reported version.
const Version = "0.1.0"

// New builds an MCP server exposing the AIS read-only tools, backed by the
// given RBHU client. The client holds the per-session OAuth token, set by the
// authorize/exchange tools.
func New(c *rbhu.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "rbhu-ais",
		Title:   "Raiffeisen Hungary (AIS, read-only)",
		Version: Version,
	}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_consent",
		Title:       "Create account-information consent",
		Description: "Create a read-only AIS consent for one or more IBANs and return the consent id plus the URL the customer must open to authorize it (SCA). Does not move money.",
	}, createConsent(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "submit_authorization_code",
		Title:       "Submit SCA authorization code",
		Description: "Exchange the authorization code (from the redirect URL after the customer approves SCA) for an access token. Call this after create_consent and the customer has authorized.",
	}, submitCode(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "authorize_in_browser",
		Title:       "Authorize via local browser (local setup only)",
		Description: "Open the SCA URL in a local browser and automatically capture the authorization code on a local callback server. Only works when the connector runs locally and the app's redirect URI points at the local callback address.",
	}, authorizeBrowser(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_accounts",
		Title:       "List accounts",
		Description: "List the accounts (current and savings) covered by an authorized consent. Requires a prior authorization.",
	}, listAccounts(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_balances",
		Title:       "Get account balances",
		Description: "Get the balances of one account under an authorized consent.",
	}, getBalances(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_transactions",
		Title:       "Get account transactions",
		Description: "Get the transactions of one account under an authorized consent. bookingStatus is booked, pending or both (default booked).",
	}, getTransactions(c))

	return s
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
}

func createConsent(c *rbhu.Client) mcp.ToolHandlerFor[CreateConsentIn, CreateConsentOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateConsentIn) (*mcp.CallToolResult, CreateConsentOut, error) {
		consent, err := c.CreateAISConsent(ctx, rbhu.AISConsentParams{
			IBANs: in.IBANs, PSUID: in.PSUID, Recurring: true,
		})
		if err != nil {
			return errResult(err), CreateConsentOut{}, nil
		}
		return nil, CreateConsentOut{
			ConsentID:    consent.ID,
			Status:       consent.Status,
			AuthorizeURL: c.AuthorizeURL(rbhu.ScopeAISP, consent.ID, ""),
		}, nil
	}
}

// ---- submit_authorization_code ----

type SubmitCodeIn struct {
	Code string `json:"code" jsonschema:"the authorization code from the redirect URL after SCA"`
}

type AuthOut struct {
	Status    string `json:"status"`
	ExpiresIn int    `json:"expires_in_seconds"`
}

func submitCode(c *rbhu.Client) mcp.ToolHandlerFor[SubmitCodeIn, AuthOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SubmitCodeIn) (*mcp.CallToolResult, AuthOut, error) {
		tok, err := c.ExchangeToken(ctx, rbhu.ScopeAISP, in.Code)
		if err != nil {
			return errResult(err), AuthOut{}, nil
		}
		return nil, AuthOut{Status: "authorized", ExpiresIn: tok.ExpiresIn}, nil
	}
}

// ---- authorize_in_browser ----

type AuthorizeBrowserIn struct {
	ConsentID    string `json:"consent_id"`
	CallbackAddr string `json:"callback_addr" jsonschema:"local host:port the redirect is captured on (default 127.0.0.1:8089)"`
}

func authorizeBrowser(c *rbhu.Client) mcp.ToolHandlerFor[AuthorizeBrowserIn, AuthOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in AuthorizeBrowserIn) (*mcp.CallToolResult, AuthOut, error) {
		addr := in.CallbackAddr
		if addr == "" {
			addr = "127.0.0.1:8089"
		}
		tok, err := c.CompleteAuthorization(ctx, rbhu.ScopeAISP, in.ConsentID, addr, openBrowser)
		if err != nil {
			return errResult(err), AuthOut{}, nil
		}
		return nil, AuthOut{Status: "authorized", ExpiresIn: tok.ExpiresIn}, nil
	}
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

func listAccounts(c *rbhu.Client) mcp.ToolHandlerFor[ConsentIn, ListAccountsOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ConsentIn) (*mcp.CallToolResult, ListAccountsOut, error) {
		accs, err := c.ListAccounts(ctx, in.ConsentID)
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
}

// ---- get_balances ----

type AccountIn struct {
	ConsentID string `json:"consent_id"`
	AccountID string `json:"account_id" jsonschema:"the account resource_id from list_accounts"`
}

type RawOut struct {
	AccountID string          `json:"account_id"`
	Data      json.RawMessage `json:"data"`
}

func getBalances(c *rbhu.Client) mcp.ToolHandlerFor[AccountIn, RawOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in AccountIn) (*mcp.CallToolResult, RawOut, error) {
		bal, err := c.Balances(ctx, in.ConsentID, in.AccountID)
		if err != nil {
			return errResult(err), RawOut{}, nil
		}
		data, _ := json.Marshal(bal)
		return nil, RawOut{AccountID: in.AccountID, Data: data}, nil
	}
}

// ---- get_transactions ----

type TransactionsIn struct {
	ConsentID     string `json:"consent_id"`
	AccountID     string `json:"account_id" jsonschema:"the account resource_id from list_accounts"`
	BookingStatus string `json:"booking_status" jsonschema:"booked, pending or both (default booked)"`
}

func getTransactions(c *rbhu.Client) mcp.ToolHandlerFor[TransactionsIn, RawOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in TransactionsIn) (*mcp.CallToolResult, RawOut, error) {
		tx, err := c.Transactions(ctx, in.ConsentID, in.AccountID, in.BookingStatus)
		if err != nil {
			return errResult(err), RawOut{}, nil
		}
		data, _ := json.Marshal(tx)
		return nil, RawOut{AccountID: in.AccountID, Data: data}, nil
	}
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
