CREATE TABLE IF NOT EXISTS context (
    id          BIGSERIAL PRIMARY KEY,
    agent_id    VARCHAR(64) NOT NULL,
    domain      VARCHAR(128) NOT NULL,
    key_name    VARCHAR(256) NOT NULL,
    value       TEXT NOT NULL,
    confidence  REAL NOT NULL DEFAULT 1.0,
    source_file VARCHAR(512) NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(domain, key_name)
);
