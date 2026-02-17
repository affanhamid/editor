#!/bin/bash
set -e

cd "$(dirname "$0")"

DB_NAME="${ARCHITECT_DB:-architect_meta}"
DB_USER="${ARCHITECT_USER:-architect}"

echo "Seeding database '$DB_NAME'..."

psql -U $DB_USER -d $DB_NAME <<SQL
-- Sample tasks
INSERT INTO tasks (title, description, status, assigned_to, risk_level) VALUES
  ('Set up PostgreSQL schema', 'Create all tables and triggers for the architect system', 'completed', 'agent-schema', 'low'),
  ('Build MCP-PG bridge', 'Create MCP server that exposes database operations', 'pending', NULL, 'medium'),
  ('Implement orchestrator', 'Build the agent orchestrator that manages task assignment', 'pending', NULL, 'high');

-- Task dependency: orchestrator blocked by MCP bridge
INSERT INTO task_edges (from_task, to_task, edge_type) VALUES (2, 3, 'blocks');

-- Sample agent
INSERT INTO agents (agent_id, pid, status, current_task_id, worktree_path) VALUES
  ('agent-schema', 1234, 'idle', NULL, '/tmp/worktrees/schema');

-- Sample messages
INSERT INTO messages (agent_id, channel, content, msg_type, ref_task_id) VALUES
  ('agent-schema', 'general', 'Schema setup complete, all tables created.', 'update', 1),
  ('agent-schema', 'general', 'Should we add partitioning to the messages table?', 'question', NULL);

-- Sample context
INSERT INTO context (agent_id, domain, key_name, value, confidence) VALUES
  ('agent-schema', 'database', 'primary_db', 'architect', 1.0),
  ('agent-schema', 'database', 'migration_count', '6', 1.0);

-- Sample decision
INSERT INTO decisions (agent_id, domain, decision, rationale, risk_level) VALUES
  ('agent-schema', 'database', 'Use BIGSERIAL for all primary keys', 'Avoids integer overflow for high-throughput tables', 'low');

-- Sample tool
INSERT INTO tool_registry (tool_name, version, spec_json, source) VALUES
  ('psql', '16.0', '{"description": "PostgreSQL CLI client"}', 'system');

SQL

echo "Seed data inserted."
