// Command rbhu-connector runs the RBHU AIS read-only MCP connector over stdio,
// for local use with Claude Desktop or Claude Code.
//
//	go build -o bin/rbhu-connector ./cmd/rbhu-connector
//
// Configure it in Claude Desktop (claude_desktop_config.json):
//
//	{
//	  "mcpServers": {
//	    "rbhu-ais": { "command": "/abs/path/bin/rbhu-connector" }
//	  }
//	}
//
// Credentials are read from secrets/.env and secrets/<cert>.p12 (override with
// RBHU_ENV_FILE and RBHU_P12).
package main

import (
	"context"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/s4mur4i/rbhu"
	"github.com/s4mur4i/rbhu/connector"
)

func main() {
	client, err := rbhu.NewSandboxFromEnv(os.Getenv("RBHU_ENV_FILE"), os.Getenv("RBHU_P12"))
	if err != nil {
		log.Fatalf("rbhu-connector: %v", err)
	}
	srv := connector.New(client)
	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("rbhu-connector: %v", err)
	}
}
