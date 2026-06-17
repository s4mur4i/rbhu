package rbhu

import "testing"

func TestEnvironmentURL(t *testing.T) {
	got := Sandbox.URL(ServiceAccounts)
	want := "https://hu-api-sandbox.raiffeisen.hu/rbhu/prod02/psd2-accounts-api-1.3.2-rbhu/v1/"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
	if got[len(got)-1] != '/' {
		t.Errorf("service URL must end with a trailing slash, got %q", got)
	}
}

func TestEnvironmentTokenURL(t *testing.T) {
	cases := map[Scope]string{
		ScopeAISP: "https://hu-api-sandbox.raiffeisen.hu/rbhu/prod02/aisp/oauth2/token",
		ScopePISP: "https://hu-api-sandbox.raiffeisen.hu/rbhu/prod02/pisp/oauth2/token",
		ScopeCISP: "https://hu-api-sandbox.raiffeisen.hu/rbhu/prod02/cisp/oauth2/token",
	}
	for scope, want := range cases {
		if got := Sandbox.TokenURL(scope.tokenSegment()); got != want {
			t.Errorf("TokenURL(%s) = %q, want %q", scope, got, want)
		}
	}
}

func TestEnvironmentAuthorizeURL(t *testing.T) {
	want := "https://hu-bridge-sandbox.raiffeisen.hu/rbhu/prod02/psd2-rbhu-bridge-api/bridge/authorize"
	if got := Sandbox.AuthorizeURL(); got != want {
		t.Fatalf("AuthorizeURL = %q, want %q", got, want)
	}
}
