#!/usr/bin/env bash
set -euo pipefail

DB_URL="${1:-postgres://architect:architect_local@localhost:5432/architect_meta?sslmode=disable}"

psql "$DB_URL" -c "TRUNCATE task_edges, messages, context, decisions, agents, tasks RESTART IDENTITY CASCADE;"
echo "All tables truncated."
