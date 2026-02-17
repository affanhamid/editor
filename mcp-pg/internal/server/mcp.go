package mcpserver

import (
	"github.com/affanhamid/editor/mcp-pg/internal/db"
	"github.com/affanhamid/editor/mcp-pg/internal/tools"
	"github.com/mark3labs/mcp-go/server"
)

func New(q *db.Queries) *server.MCPServer {
	s := server.NewMCPServer(
		"mcp-pg",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	cfg := tools.NewConfig(q)
	tools.RegisterAll(s, cfg)

	return s
}
