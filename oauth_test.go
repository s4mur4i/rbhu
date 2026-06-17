package rbhu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func testClient(srv *httptest.Server) *Client {
	return New(
		Config{ClientID: "cid", ClientSecret: "csecret", RedirectURL: "https://app.test/cb"},
		WithEnvironment(Environment{APIBase: srv.URL, BridgeBase: srv.URL}),
		WithHTTPClient(srv.Client()),
	)
}

func TestAuthorizeURL(t *testing.T) {
	c := New(Config{ClientID: "cid", RedirectURL: "https://app.test/cb"})
	raw := c.AuthorizeURL(ScopeAISP, "consent-123", "st8")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	for k, want := range map[string]string{
		"response_type": "code",
		"scope":         "AISP",
		"redirect_uri":  "https://app.test/cb",
		"client_id":     "cid",
		"consentId":     "consent-123",
		"state":         "st8",
	} {
		if q.Get(k) != want {
			t.Errorf("query %q = %q, want %q", k, q.Get(k), want)
		}
	}
}

func TestExchangeToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/aisp/oauth2/token" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("content-type = %q", ct)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "the-code" {
			t.Errorf("form = %v", r.Form)
		}
		if r.Form.Get("client_id") != "cid" || r.Form.Get("client_secret") != "csecret" {
			t.Errorf("missing client creds: %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token_type":"bearer","access_token":"AT","refresh_token":"RT","expires_in":300}`))
	}))
	defer srv.Close()

	c := testClient(srv)
	tok, err := c.ExchangeToken(context.Background(), ScopeAISP, "the-code")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "AT" || tok.RefreshToken != "RT" || tok.ExpiresIn != 300 {
		t.Fatalf("token = %+v", tok)
	}
	if tok.Expiry.IsZero() {
		t.Error("expiry not computed")
	}
	if tok.Expired() {
		t.Error("fresh token reported expired")
	}
	if c.CurrentToken() == nil || c.CurrentToken().AccessToken != "AT" {
		t.Error("token not stored on client")
	}
}

func TestExchangeTokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorCode":"invalid_client","errorDescription":"bad secret"}`))
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.ExchangeToken(context.Background(), ScopeAISP, "x")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if apiErr.StatusCode != 401 || apiErr.ErrorCode != "invalid_client" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestTokenExpired(t *testing.T) {
	if !(*Token)(nil).Expired() {
		t.Error("nil token should be expired")
	}
	if (&Token{}).Expired() {
		t.Error("zero-expiry token should not be expired")
	}
}
