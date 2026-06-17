package rbhu

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/s4mur4i/rbhu/psd2/accounts"
)

func TestNewBuildsMTLSClient(t *testing.T) {
	cert := &tls.Certificate{}
	c := New(Config{ClientID: "x", ClientSecret: "y", Certificate: cert})
	tr, ok := c.HTTPClient().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", c.HTTPClient().Transport)
	}
	if len(tr.TLSClientConfig.Certificates) != 1 {
		t.Errorf("expected 1 client certificate, got %d", len(tr.TLSClientConfig.Certificates))
	}
}

func TestRequestEditorInjectsHeaders(t *testing.T) {
	c := New(Config{ClientID: "x", ClientSecret: "y"})
	c.SetToken(&Token{AccessToken: "AT", TokenType: "bearer"})

	req, _ := http.NewRequest(http.MethodGet, "https://x.test/", nil)
	if err := c.requestEditor(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer AT" {
		t.Errorf("Authorization = %q", got)
	}
	if req.Header.Get("X-Request-ID") == "" {
		t.Error("X-Request-ID not set")
	}
	if req.Header.Get("Date") == "" {
		t.Error("Date not set")
	}

	// Existing X-Request-ID must be preserved.
	req2, _ := http.NewRequest(http.MethodGet, "https://x.test/", nil)
	req2.Header.Set("X-Request-ID", "keep-me")
	_ = c.requestEditor(context.Background(), req2)
	if req2.Header.Get("X-Request-ID") != "keep-me" {
		t.Errorf("X-Request-ID overwritten: %q", req2.Header.Get("X-Request-ID"))
	}
}

func TestAccountsClientWiring(t *testing.T) {
	var gotAuth, gotReqID, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotReqID = r.Header.Get("X-Request-ID")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accounts":[]}`))
	}))
	defer srv.Close()

	c := New(
		Config{ClientID: "cid", ClientSecret: "sec"},
		WithEnvironment(Environment{APIBase: srv.URL, BridgeBase: srv.URL}),
		WithHTTPClient(srv.Client()),
		WithToken(&Token{AccessToken: "AT"}),
	)

	api, err := c.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := api.GetAccountListWithResponse(context.Background(), &accounts.GetAccountListParams{
		XRequestID: openapi_types.UUID{},
		ConsentID:  "consent-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != http.StatusOK {
		t.Errorf("status = %d, body=%s", resp.StatusCode(), string(resp.Body))
	}
	if gotAuth != "Bearer AT" {
		t.Errorf("Authorization forwarded = %q", gotAuth)
	}
	if gotReqID == "" {
		t.Error("X-Request-ID not forwarded")
	}
	if !strings.Contains(gotPath, "psd2-accounts-api-1.3.2-rbhu/v1") {
		t.Errorf("path = %q", gotPath)
	}
}
