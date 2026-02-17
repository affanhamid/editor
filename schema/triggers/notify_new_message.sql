CREATE OR REPLACE FUNCTION notify_new_message() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('agent_messages', json_build_object(
        'id', NEW.id,
        'agent_id', NEW.agent_id,
        'channel', NEW.channel,
        'msg_type', NEW.msg_type
    )::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_message_notify ON messages;
CREATE TRIGGER trg_message_notify AFTER INSERT ON messages FOR EACH ROW EXECUTE FUNCTION notify_new_message();
