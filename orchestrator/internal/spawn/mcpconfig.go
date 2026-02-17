package spawn

import (
	"encoding/json"
)

// GenerateMCPConfig creates a per-agent .mcp.json.
func GenerateMCPConfig(agentID string, branchName string, mcpPgBinaryPath string, dbURL string) ([]byte, error) {
	config := map[string]any{
		"mcpServers": map[string]any{
			"architect-pg": map[string]any{
				"command": mcpPgBinaryPath,
				"env": map[string]string{
					"ARCHITECT_AGENT_ID": agentID,
					"ARCHITECT_BRANCH":   branchName,
					"ARCHITECT_DB_URL":   dbURL,
				},
			},
		},
	}
	return json.MarshalIndent(config, "", "  ")
}
