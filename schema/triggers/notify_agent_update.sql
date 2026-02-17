CREATE OR REPLACE FUNCTION notify_agent_update() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('agent_updates', json_build_object(
        'agent_id', NEW.agent_id,
        'status', NEW.status,
        'current_task_id', NEW.current_task_id
    )::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_agent_notify ON agents;
CREATE TRIGGER trg_agent_notify AFTER INSERT OR UPDATE ON agents FOR EACH ROW EXECUTE FUNCTION notify_agent_update();
