// Package rbhu is a Go client for the Raiffeisen Bank Zrt. (Hungary) PSD2 Open
// Banking APIs, compliant with the Berlin Group NextGenPSD2 standard 1.3.2.
//
// It wraps typed clients generated from the official OpenAPI specifications
// (see the psd2/ subpackages) with an ergonomic layer handling configuration,
// mutual-TLS, the OAuth bridge token flow and Berlin Group request headers.
//
// The default target is the RBHU sandbox (Sandbox).
package rbhu

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Client is the entry point for talking to RBHU. It is safe for concurrent use.
type Client struct {
	env        Environment
	cfg        Config
	httpClient *http.Client

	mu    sync.RWMutex
	token *Token
}

// Option configures a Client.
type Option func(*Client)

// WithEnvironment overrides the target environment (default: Sandbox).
func WithEnvironment(e Environment) Option {
	return func(c *Client) { c.env = e }
}

// WithHTTPClient sets the underlying *http.Client. When set, the caller is
// responsible for any TLS/mTLS configuration.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// WithToken seeds an already-obtained OAuth token.
func WithToken(t *Token) Option {
	return func(c *Client) { c.token = t }
}

// New creates a Client. If cfg carries a Certificate and no explicit HTTP
// client is supplied, a mutual-TLS client is built automatically.
func New(cfg Config, opts ...Option) *Client {
	c := &Client{
		env: Sandbox,
		cfg: cfg,
	}
	for _, o := range opts {
		o(c)
	}
	if c.httpClient == nil {
		c.httpClient = newHTTPClient(cfg.Certificate)
	}
	return c
}

func newHTTPClient(cert *tls.Certificate) *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		Proxy:           http.ProxyFromEnvironment,
	}
	if cert != nil {
		tr.TLSClientConfig.Certificates = []tls.Certificate{*cert}
	}
	return &http.Client{Transport: tr, Timeout: 60 * time.Second}
}

// HTTPClient exposes the underlying *http.Client (e.g. to pass to a generated
// client's WithHTTPClient option).
func (c *Client) HTTPClient() *http.Client { return c.httpClient }

// Environment returns the configured environment.
func (c *Client) Environment() Environment { return c.env }

// SetToken stores the active OAuth token used to authenticate data calls.
func (c *Client) SetToken(t *Token) {
	c.mu.Lock()
	c.token = t
	c.mu.Unlock()
}

// CurrentToken returns the active token, or nil.
func (c *Client) CurrentToken() *Token {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

// userAgent is sent on every request; the default Go user agent is WAF-blocked.
const userAgent = "rbhu-go/0.1"

// newRequestID returns a fresh X-Request-ID value.
func newRequestID() string { return uuid.NewString() }

// requestEditor injects the cross-cutting Berlin Group headers that are not
// modelled as per-operation parameters: a bearer Authorization header, a Date,
// and a fallback X-Request-ID. It matches the generated RequestEditorFn type.
func (c *Client) requestEditor(_ context.Context, req *http.Request) error {
	if t := c.CurrentToken(); t != nil && t.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+t.AccessToken)
	}
	// The gateway's key-auth requires the marketplace client id on every call
	// (in addition to any bearer token); without it data APIs return 401
	// "No API key found in request".
	if c.cfg.ClientID != "" && req.Header.Get("client_id") == "" {
		req.Header.Set("client_id", c.cfg.ClientID)
	}
	if req.Header.Get("X-Request-ID") == "" {
		req.Header.Set("X-Request-ID", uuid.NewString())
	}
	if req.Header.Get("Date") == "" {
		req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	}
	// The RBHU gateway requires an explicit JSON Accept; without it requests
	// are rejected (HTTP 406).
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	// The default "Go-http-client" User-Agent is blocked by the gateway WAF
	// (HTTP 403); send our own.
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}
	return nil
}
