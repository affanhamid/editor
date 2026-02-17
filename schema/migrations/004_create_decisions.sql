CREATE TABLE IF NOT EXISTS decisions (
    id          BIGSERIAL PRIMARY KEY,
    agent_id    VARCHAR(64) NOT NULL,
    git_sha     VARCHAR(40) NULL,
    branch      VARCHAR(128),
    domain      VARCHAR(128) NOT NULL,
    decision    TEXT NOT NULL,
    rationale   TEXT NOT NULL,
    alternatives_considered TEXT NULL,
    risk_level  VARCHAR(16) NOT NULL DEFAULT 'low' CHECK (risk_level IN ('low', 'medium', 'high')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_decisions_domain ON decisions(domain, created_at);
