package rbhu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateAISConsent(t *testing.T) {
	var gotBody map[string]any
	var gotClientID, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClientID = r.Header.Get("client_id")
		gotAccept = r.Header.Get("Accept")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"consentStatus":"received","consentId":"c-1","_links":{"scaRedirect":{"href":"https://bridge/authorize?x=1"},"status":{"href":"https://x/status"}}}`))
	}))
	defer srv.Close()

	c := New(Config{ClientID: "cid", ClientSecret: "s"},
		WithEnvironment(Environment{APIBase: srv.URL, BridgeBase: srv.URL}),
		WithHTTPClient(srv.Client()))

	got, err := c.CreateAISConsent(context.Background(), AISConsentParams{
		IBANs: []string{"HU19120010080010059400100008"},
		PSUID: "82742150",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "c-1" || got.Status != "received" {
		t.Fatalf("consent = %+v", got)
	}
	if got.SCARedirect != "https://bridge/authorize?x=1" {
		t.Errorf("scaRedirect = %q", got.SCARedirect)
	}
	if gotClientID != "cid" {
		t.Errorf("client_id header = %q", gotClientID)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q", gotAccept)
	}
	// Body carries the dedicated account and defaults.
	access, _ := gotBody["access"].(map[string]any)
	if access == nil || access["accounts"] == nil {
		t.Errorf("body access missing accounts: %v", gotBody)
	}
	if gotBody["frequencyPerDay"].(float64) != 4 {
		t.Errorf("default frequencyPerDay not applied: %v", gotBody["frequencyPerDay"])
	}
}

func TestCreateAISConsentRequiresIBAN(t *testing.T) {
	c := New(Config{ClientID: "x", ClientSecret: "y"})
	if _, err := c.CreateAISConsent(context.Background(), AISConsentParams{}); err == nil {
		t.Fatal("expected error with no IBANs")
	}
}

func TestListAccounts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer AT" {
			t.Errorf("missing bearer: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Consent-ID") != "c-1" {
			t.Errorf("Consent-ID = %q", r.Header.Get("Consent-ID"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accounts":[{"resourceId":"a1","iban":"HU19","currency":"HUF"}]}`))
	}))
	defer srv.Close()

	c := New(Config{ClientID: "cid", ClientSecret: "s"},
		WithEnvironment(Environment{APIBase: srv.URL, BridgeBase: srv.URL}),
		WithHTTPClient(srv.Client()), WithToken(&Token{AccessToken: "AT"}))

	accs, err := c.ListAccounts(context.Background(), "c-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(accs) != 1 || accs[0].Currency != "HUF" {
		t.Fatalf("accounts = %+v", accs)
	}
}

func TestListAccountsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"tppMessages":[{"category":"ERROR","code":"TOKEN_INVALID","text":"nope"}]}`))
	}))
	defer srv.Close()

	c := New(Config{ClientID: "cid", ClientSecret: "s"},
		WithEnvironment(Environment{APIBase: srv.URL, BridgeBase: srv.URL}),
		WithHTTPClient(srv.Client()), WithToken(&Token{AccessToken: "AT"}))

	_, err := c.ListAccounts(context.Background(), "c-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if ae, ok := err.(*APIError); !ok || ae.StatusCode != 401 {
		t.Fatalf("err = %v (%T)", err, err)
	}
}

func TestListenForCode(t *testing.T) {
	c := New(Config{ClientID: "cid", ClientSecret: "s"})
	cl, err := c.ListenForCode("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	go func() {
		resp, err := http.Get("http://" + cl.Addr() + "/callback?code=the-code&state=st1")
		if err == nil {
			resp.Body.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cl.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Code != "the-code" || res.State != "st1" {
		t.Fatalf("authResult = %+v", res)
	}
}
