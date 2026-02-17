package spawn

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ResolveMCPPgBinary finds the mcp-pg binary by checking:
// 1. The explicitly provided path (if non-empty and exists)
// 2. ~/.architect/bin/architect-mcp-pg (installed location)
// 3. Co-located with the running orchestrator binary
func ResolveMCPPgBinary(explicit string) string {
	// If explicitly provided and exists, use it.
	if explicit != "" {
		if abs, err := filepath.Abs(explicit); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}

	// Check installed location.
	if home, err := os.UserHomeDir(); err == nil {
		installed := filepath.Join(home, ".architect", "bin", "architect-mcp-pg")
		if _, err := os.Stat(installed); err == nil {
			return installed
		}
	}

	// Fallback: same directory as the running orchestrator binary.
	if exe, err := os.Executable(); err == nil {
		colocated := filepath.Join(filepath.Dir(exe), "architect-mcp-pg")
		if _, err := os.Stat(colocated); err == nil {
			return colocated
		}
	}

	// Last resort: return the explicit path as-is (will fail at spawn time with a clear error).
	if explicit != "" {
		return explicit
	}
	return "architect-mcp-pg"
}

// GenerateMCPConfig creates a per-agent .mcp.json.
func GenerateMCPConfig(agentID string, branchName string, mcpPgBinaryPath string, dbURL string) ([]byte, error) {
	absPath, err := filepath.Abs(mcpPgBinaryPath)
	if err != nil {
		return nil, err
	}
	config := map[string]any{
		"mcpServers": map[string]any{
			"architect-pg": map[string]any{
				"command": absPath,
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
