CREATE TABLE IF NOT EXISTS tasks (
    id          BIGSERIAL PRIMARY KEY,
    parent_id   BIGINT NULL REFERENCES tasks(id),
    title       VARCHAR(512) NOT NULL,
    description TEXT NOT NULL,
    status      VARCHAR(32) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'completed', 'failed', 'blocked')),
    assigned_to VARCHAR(64) NULL,
    risk_level  VARCHAR(16) NOT NULL DEFAULT 'low' CHECK (risk_level IN ('low', 'medium', 'high')),
    output      TEXT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS task_edges (
    from_task   BIGINT NOT NULL REFERENCES tasks(id),
    to_task     BIGINT NOT NULL REFERENCES tasks(id),
    edge_type   VARCHAR(16) NOT NULL DEFAULT 'blocks' CHECK (edge_type IN ('blocks', 'informs')),
    PRIMARY KEY (from_task, to_task)
);
