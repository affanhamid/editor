#!/bin/bash
cd "$(dirname "$0")"
DB_NAME="${ARCHITECT_DB:-architect}"
DB_ADMIN="${ARCHITECT_ADMIN:-$(whoami)}"
echo "Dropping database '$DB_NAME'..."
psql -U $DB_ADMIN -d postgres -c "DROP DATABASE IF EXISTS $DB_NAME;"
echo "Done."
