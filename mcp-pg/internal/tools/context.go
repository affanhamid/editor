package tools

import (
	"context"
	"fmt"

	"github.com/affanhamid/editor/mcp-pg/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerContextTools(s *server.MCPServer, cfg *Config) {
	readContext := mcp.NewTool("read_context",
		mcp.WithDescription("Read shared project knowledge written by all agents. Use this on startup to understand what other agents have discovered about the codebase."),
		mcp.WithString("domain",
			mcp.Description("Filter by domain (e.g., 'auth', 'database', 'api'). Omit to read all domains."),
		),
	)

	writeContext := mcp.NewTool("write_context",
		mcp.WithDescription("Write a discovery to the shared knowledge base. Other agents will see this immediately. Use this whenever you learn something about the codebase that other agents might need."),
		mcp.WithString("domain",
			mcp.Description("Knowledge domain: 'auth', 'database', 'api', 'frontend', 'infra', etc."),
			mcp.Required(),
		),
		mcp.WithString("key",
			mcp.Description("What was discovered, e.g., 'jwt_algorithm', 'orm_library', 'api_framework'"),
			mcp.Required(),
		),
		mcp.WithString("value",
			mcp.Description("The discovered value, e.g., 'RS256', 'GORM', 'Gin'"),
			mcp.Required(),
		),
		mcp.WithNumber("confidence",
			mcp.Description("How confident you are (0.0 to 1.0)"),
			mcp.DefaultNumber(1.0),
		),
		mcp.WithString("source_file",
			mcp.Description("File path where this was discovered"),
		),
	)

	s.AddTool(readContext, makeReadContextHandler(cfg))
	s.AddTool(writeContext, makeWriteContextHandler(cfg))
}

func makeReadContextHandler(cfg *Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var domain *string
		if v := request.GetString("domain", ""); v != "" {
			domain = &v
		}

		entries, err := cfg.Queries.ReadContext(ctx, domain)
		if err != nil {
			return errorResult(err), nil
		}

		return textResult(db.ToJSON(entries)), nil
	}
}

func makeWriteContextHandler(cfg *Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		domain := request.GetString("domain", "")
		key := request.GetString("key", "")
		value := request.GetString("value", "")
		confidence := request.GetFloat("confidence", 1.0)

		var sourceFile *string
		if v := request.GetString("source_file", ""); v != "" {
			sourceFile = &v
		}

		id, err := cfg.Queries.WriteContext(ctx, cfg.AgentID, domain, key, value, confidence, sourceFile)
		if err != nil {
			return errorResult(err), nil
		}

		return textResult(fmt.Sprintf("Context written: id=%d, domain=%s, key=%s", id, domain, key)), nil
	}
}
