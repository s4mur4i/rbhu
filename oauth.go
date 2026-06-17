package rbhu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Scope is an OAuth/PSD2 access scope.
type Scope string

const (
	ScopeAISP Scope = "AISP" // account information
	ScopePISP Scope = "PISP" // payment initiation
	ScopeCISP Scope = "CISP" // confirmation of funds
)

// tokenPath maps a scope to its oauth2 token path segment.
func (s Scope) tokenSegment() string {
	switch s {
	case ScopePISP:
		return "pisp"
	case ScopeCISP:
		return "cisp"
	default:
		return "aisp"
	}
}

// Token is an OAuth2 access token returned by the token endpoint.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`

	// Expiry is computed at fetch time from ExpiresIn.
	Expiry time.Time `json:"-"`
}

// Expired reports whether the token is at or past its expiry (with a small
// safety margin). A zero Expiry is treated as not expired.
func (t *Token) Expired() bool {
	if t == nil {
		return true
	}
	if t.Expiry.IsZero() {
		return false
	}
	return time.Now().After(t.Expiry.Add(-30 * time.Second))
}

// AuthorizeURL builds the OAuth bridge authorization URL the PSU must visit to
// grant consent and perform SCA. state may be empty.
func (c *Client) AuthorizeURL(scope Scope, consentID, state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("scope", string(scope))
	q.Set("redirect_uri", c.cfg.RedirectURL)
	q.Set("client_id", c.cfg.ClientID)
	q.Set("consentId", consentID)
	if state != "" {
		q.Set("state", state)
	}
	return c.env.AuthorizeURL() + "?" + q.Encode()
}

// ExchangeToken exchanges an authorization code for an access token at the
// scope's oauth2 token endpoint, stores it on the client and returns it.
func (c *Client) ExchangeToken(ctx context.Context, scope Scope, code string) (*Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("code", code)
	if c.cfg.RedirectURL != "" {
		form.Set("redirect_uri", c.cfg.RedirectURL)
	}
	return c.postToken(ctx, scope, form)
}

// RefreshToken obtains a fresh access token using a refresh token.
func (c *Client) RefreshToken(ctx context.Context, scope Scope, refreshToken string) (*Token, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("refresh_token", refreshToken)
	return c.postToken(ctx, scope, form)
}

func (c *Client) postToken(ctx context.Context, scope Scope, form url.Values) (*Token, error) {
	endpoint := c.env.TokenURL(scope.tokenSegment())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rbhu: token request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, resp.Header.Get("X-Request-ID"), body)
	}

	var t Token
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("rbhu: decode token: %w", err)
	}
	if t.ExpiresIn > 0 {
		t.Expiry = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	}
	c.SetToken(&t)
	return &t, nil
}
