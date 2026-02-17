package tools

import (
	"context"
	"fmt"

	"github.com/affanhamid/editor/mcp-pg/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerDecisionTools(s *server.MCPServer, cfg *Config) {
	writeDecision := mcp.NewTool("write_decision",
		mcp.WithDescription("Record an architectural decision. Always check_decisions first to avoid conflicts."),
		mcp.WithString("domain",
			mcp.Description("Decision domain: 'auth', 'database', 'api', etc."),
			mcp.Required(),
		),
		mcp.WithString("decision",
			mcp.Description("What was decided"),
			mcp.Required(),
		),
		mcp.WithString("rationale",
			mcp.Description("Why this decision was made"),
			mcp.Required(),
		),
		mcp.WithString("alternatives",
			mcp.Description("What alternatives were considered"),
		),
		mcp.WithString("risk_level",
			mcp.Description("Risk level of the decision"),
			mcp.Enum("low", "medium", "high"),
			mcp.DefaultString("low"),
		),
		mcp.WithString("git_sha",
			mcp.Description("Git commit SHA associated with this decision"),
		),
	)

	checkDecisions := mcp.NewTool("check_decisions",
		mcp.WithDescription("Check existing architectural decisions for a domain. ALWAYS call this before write_decision to avoid conflicts."),
		mcp.WithString("domain",
			mcp.Description("The domain to check"),
			mcp.Required(),
		),
	)

	s.AddTool(writeDecision, makeWriteDecisionHandler(cfg))
	s.AddTool(checkDecisions, makeCheckDecisionsHandler(cfg))
}

func makeWriteDecisionHandler(cfg *Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		domain := request.GetString("domain", "")
		decision := request.GetString("decision", "")
		rationale := request.GetString("rationale", "")
		riskLevel := request.GetString("risk_level", "low")

		var alternatives *string
		if v := request.GetString("alternatives", ""); v != "" {
			alternatives = &v
		}

		var gitSHA *string
		if v := request.GetString("git_sha", ""); v != "" {
			gitSHA = &v
		}

		id, err := cfg.Queries.WriteDecision(ctx, cfg.AgentID, cfg.Branch, domain, decision, rationale, alternatives, riskLevel, gitSHA)
		if err != nil {
			return errorResult(err), nil
		}

		return textResult(fmt.Sprintf("Decision recorded: id=%d, domain=%s", id, domain)), nil
	}
}

func makeCheckDecisionsHandler(cfg *Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		domain := request.GetString("domain", "")
		if domain == "" {
			return errorResult(fmt.Errorf("domain is required")), nil
		}

		decisions, err := cfg.Queries.CheckDecisions(ctx, domain)
		if err != nil {
			return errorResult(err), nil
		}

		return textResult(db.ToJSON(decisions)), nil
	}
}
