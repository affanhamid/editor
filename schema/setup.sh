#!/bin/bash
set -e

cd "$(dirname "$0")"

DB_NAME="${ARCHITECT_DB:-architect}"
DB_USER="${ARCHITECT_USER:-architect}"
DB_PASSWORD="${ARCHITECT_PASSWORD:-architect_local}"
DB_ADMIN="${ARCHITECT_ADMIN:-$(whoami)}"

echo "Creating database '$DB_NAME' and user '$DB_USER'..."

psql -U $DB_ADMIN -d postgres <<SQL
  SELECT 'CREATE DATABASE $DB_NAME' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$DB_NAME')\gexec
  DO \$\$
  BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '$DB_USER') THEN
      CREATE ROLE $DB_USER WITH LOGIN PASSWORD '$DB_PASSWORD';
    END IF;
  END
  \$\$;
  GRANT ALL PRIVILEGES ON DATABASE $DB_NAME TO $DB_USER;
  ALTER DATABASE $DB_NAME OWNER TO $DB_USER;
SQL

echo "Running migrations..."
for f in migrations/*.sql; do
  echo "  Running $f..."
  psql -U $DB_USER -d $DB_NAME -f "$f"
done

echo "Installing triggers..."
for f in triggers/*.sql; do
  echo "  Running $f..."
  psql -U $DB_USER -d $DB_NAME -f "$f"
done

echo "Done. Database '$DB_NAME' is ready."
