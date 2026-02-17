CREATE TABLE IF NOT EXISTS messages (
    id          BIGSERIAL PRIMARY KEY,
    agent_id    VARCHAR(64) NOT NULL,
    channel     VARCHAR(64) NOT NULL DEFAULT 'general',
    content     TEXT NOT NULL,
    msg_type    VARCHAR(32) NOT NULL CHECK (msg_type IN ('update', 'question', 'answer', 'blocker', 'discovery', 'decision')),
    ref_task_id BIGINT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_messages_channel ON messages(channel, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_agent ON messages(agent_id, created_at);
