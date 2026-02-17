CREATE OR REPLACE FUNCTION notify_context_change() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('context_updates', json_build_object(
        'domain', NEW.domain,
        'key_name', NEW.key_name,
        'agent_id', NEW.agent_id
    )::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_context_notify ON context;
CREATE TRIGGER trg_context_notify AFTER INSERT OR UPDATE ON context FOR EACH ROW EXECUTE FUNCTION notify_context_change();
