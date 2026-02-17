CREATE TABLE IF NOT EXISTS agents (
    agent_id        VARCHAR(64) PRIMARY KEY,
    pid             INT NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'starting' CHECK (status IN ('starting', 'idle', 'working', 'blocked', 'dead')),
    current_task_id BIGINT NULL REFERENCES tasks(id),
    worktree_path   VARCHAR(512) NULL,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_heartbeat  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
