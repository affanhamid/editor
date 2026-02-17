CREATE TABLE IF NOT EXISTS tool_registry (
    tool_name       VARCHAR(256) PRIMARY KEY,
    version         VARCHAR(64) NOT NULL,
    spec_json       JSONB NOT NULL,
    source          VARCHAR(256) NOT NULL DEFAULT 'manual',
    last_verified   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
