package tools

import (
	"context"
	"fmt"

	"github.com/affanhamid/editor/mcp-pg/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerAgentTools(s *server.MCPServer, cfg *Config) {
	heartbeat := mcp.NewTool("heartbeat",
		mcp.WithDescription("Signal that you are still alive and working. Call this every few minutes."),
	)

	getAgents := mcp.NewTool("get_agents",
		mcp.WithDescription("See which other agents are currently active and what they're working on."),
		mcp.WithString("status",
			mcp.Description("Filter by status"),
		),
	)

	s.AddTool(heartbeat, makeHeartbeatHandler(cfg))
	s.AddTool(getAgents, makeGetAgentsHandler(cfg))
}

func makeHeartbeatHandler(cfg *Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		err := cfg.Queries.Heartbeat(ctx, cfg.AgentID)
		if err != nil {
			return errorResult(err), nil
		}
		return textResult(fmt.Sprintf("Heartbeat recorded for agent %s", cfg.AgentID)), nil
	}
}

func makeGetAgentsHandler(cfg *Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var status *string
		if v := request.GetString("status", ""); v != "" {
			status = &v
		}

		agents, err := cfg.Queries.GetAgents(ctx, status)
		if err != nil {
			return errorResult(err), nil
		}

		return textResult(db.ToJSON(agents)), nil
	}
}
