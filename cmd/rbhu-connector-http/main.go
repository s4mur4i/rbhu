// Command rbhu-connector-http serves the RBHU AIS read-only MCP connector over
// Streamable HTTP, for use as a remote claude.ai custom connector.
//
//	go build -o bin/rbhu-connector-http ./cmd/rbhu-connector-http
//	RBHU_CONNECTOR_TOKEN=$(openssl rand -hex 32) \
//	RBHU_CONNECTOR_ADDR=127.0.0.1:8080 ./bin/rbhu-connector-http
//
// The endpoint requires a bearer token (RBHU_CONNECTOR_TOKEN); the server
// refuses to start without one. It binds to loopback by default. Expose it over
// HTTPS via a tunnel and configure the same bearer token in the claude.ai
// connector settings.
//
// Dev scope: a single shared RBHU client/session (one set of sandbox
// credentials), gated by one operator bearer token. Per-user OAuth is a
// production follow-up — see docs/connector.md.
package main

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/s4mur4i/rbhu"
	"github.com/s4mur4i/rbhu/connector"
)

func main() {
	addr := os.Getenv("RBHU_CONNECTOR_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	token := os.Getenv("RBHU_CONNECTOR_TOKEN")
	if token == "" {
		log.Fatal("rbhu-connector-http: RBHU_CONNECTOR_TOKEN must be set (the connector exposes account data and refuses to start unauthenticated)")
	}

	client, err := rbhu.NewSandboxFromEnv(os.Getenv("RBHU_ENV_FILE"), os.Getenv("RBHU_P12"))
	if err != nil {
		log.Fatalf("rbhu-connector-http: %v", err)
	}
	// HTTP transport: no browser-based SCA tool (remote callers must not trigger
	// server-side browser launches / local binds).
	srv := connector.New(client)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", requireBearer(token, handler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	log.Printf("rbhu-connector-http listening on %s (MCP at /mcp, bearer auth required)", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("rbhu-connector-http: %v", err)
	}
}

// requireBearer rejects requests without a matching Authorization: Bearer token.
func requireBearer(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if len(got) != len(want) || subtle.ConstantTimeCompare(got, want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
