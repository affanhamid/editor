package tools

import (
	"os"

	"github.com/affanhamid/editor/mcp-pg/internal/db"
	"github.com/mark3labs/mcp-go/server"
)

type Config struct {
	AgentID string
	Branch  string
	Queries *db.Queries
}

func NewConfig(q *db.Queries) *Config {
	agentID := os.Getenv("ARCHITECT_AGENT_ID")
	if agentID == "" {
		agentID = "anonymous"
	}
	branch := os.Getenv("ARCHITECT_BRANCH")
	return &Config{
		AgentID: agentID,
		Branch:  branch,
		Queries: q,
	}
}

func RegisterAll(s *server.MCPServer, cfg *Config) {
	registerMessageTools(s, cfg)
	registerContextTools(s, cfg)
	registerTaskTools(s, cfg)
	registerDecisionTools(s, cfg)
	registerAgentTools(s, cfg)
}
