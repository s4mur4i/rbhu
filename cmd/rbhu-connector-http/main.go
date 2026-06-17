// Command rbhu-connector-http serves the RBHU AIS read-only MCP connector over
// Streamable HTTP, for use as a remote claude.ai custom connector.
//
//	go build -o bin/rbhu-connector-http ./cmd/rbhu-connector-http
//	RBHU_CONNECTOR_ADDR=:8080 ./bin/rbhu-connector-http
//
// Add it in claude.ai → Settings → Connectors as the MCP endpoint URL
// (e.g. https://<your-tunnel>/mcp). For local testing, expose it with a tunnel
// (ngrok/cloudflared).
//
// Dev scope: a single shared RBHU client/session (one set of sandbox
// credentials). Per-user OAuth (one token per claude.ai user) is a production
// follow-up — see docs/connector.md.
package main

import (
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
		addr = ":8080"
	}

	client, err := rbhu.NewSandboxFromEnv(os.Getenv("RBHU_ENV_FILE"), os.Getenv("RBHU_P12"))
	if err != nil {
		log.Fatalf("rbhu-connector-http: %v", err)
	}
	srv := connector.New(client)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	log.Printf("rbhu-connector-http listening on %s (MCP at /mcp)", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("rbhu-connector-http: %v", err)
	}
}
