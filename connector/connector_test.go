package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/s4mur4i/rbhu"
)

// fakeRBHU serves the minimal RBHU endpoints the connector exercises.
func fakeRBHU(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/oauth2/token"):
			_, _ = w.Write([]byte(`{"token_type":"bearer","access_token":"AT","expires_in":300}`))
		case strings.HasSuffix(r.URL.Path, "/consents"):
			_, _ = w.Write([]byte(`{"consentStatus":"received","consentId":"c-1","_links":{"scaRedirect":{"href":"https://bridge/x"},"status":{"href":"https://x"}}}`))
		case strings.HasSuffix(r.URL.Path, "/accounts"):
			_, _ = w.Write([]byte(`{"accounts":[{"resourceId":"sav-1","iban":"HU99","currency":"HUF","cashAccountType":"SVGS","name":"Savings"},{"resourceId":"cur-1","iban":"HU19","currency":"HUF","cashAccountType":"CACC"}]}`))
		default:
			t.Logf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func connect(t *testing.T, srv *httptest.Server) *mcp.ClientSession {
	t.Helper()
	client := rbhu.New(
		rbhu.Config{ClientID: "cid", ClientSecret: "sec", RedirectURL: "http://127.0.0.1/cb"},
		rbhu.WithEnvironment(rbhu.Environment{APIBase: srv.URL, BridgeBase: srv.URL}),
		rbhu.WithHTTPClient(srv.Client()),
	)
	server := New(client, WithBrowserAuth())

	st, ct := mcp.NewInMemoryTransports()
	if _, err := server.Connect(context.Background(), st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	mc := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := mc.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, out any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		var msg string
		if len(res.Content) > 0 {
			if tc, ok := res.Content[0].(*mcp.TextContent); ok {
				msg = tc.Text
			}
		}
		t.Fatalf("%s returned error: %s", name, msg)
	}
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("%s decode: %v (raw %s)", name, err, string(b))
	}
}

func TestConnectorListsTools(t *testing.T) {
	srv := fakeRBHU(t)
	defer srv.Close()
	cs := connect(t, srv)

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"create_consent": false, "submit_authorization_code": false,
		"authorize_in_browser": false, "list_accounts": false,
		"get_balances": false, "get_transactions": false,
	}
	for _, tool := range res.Tools {
		want[tool.Name] = true
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q not registered", name)
		}
	}
}

func TestConnectorFlow(t *testing.T) {
	srv := fakeRBHU(t)
	defer srv.Close()
	cs := connect(t, srv)

	var consent CreateConsentOut
	call(t, cs, "create_consent", map[string]any{
		"ibans": []string{"HU19120010080010059400100008"}, "psu_id": "82742150",
	}, &consent)
	if consent.ConsentID != "c-1" || consent.AuthorizeURL == "" || consent.State == "" {
		t.Fatalf("create_consent out = %+v", consent)
	}

	// Wrong state must be rejected (CSRF protection).
	resBad, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "submit_authorization_code",
		Arguments: map[string]any{"consent_id": "c-1", "code": "the-code", "state": "wrong"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resBad.IsError {
		t.Fatal("expected state-mismatch error")
	}

	var auth AuthOut
	call(t, cs, "submit_authorization_code", map[string]any{
		"consent_id": "c-1", "code": "the-code", "state": consent.State,
	}, &auth)
	if auth.Status != "authorized" {
		t.Fatalf("auth out = %+v", auth)
	}

	var accts ListAccountsOut
	call(t, cs, "list_accounts", map[string]any{"consent_id": "c-1"}, &accts)
	if len(accts.Accounts) != 2 {
		t.Fatalf("accounts = %+v", accts)
	}
	var savings bool
	for _, a := range accts.Accounts {
		if a.CashAccountType == "SVGS" {
			savings = true
		}
	}
	if !savings {
		t.Error("expected a savings (SVGS) account in the list")
	}
}
