CREATE OR REPLACE FUNCTION notify_task_update() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('task_updates', json_build_object(
        'id', NEW.id,
        'status', NEW.status,
        'assigned_to', NEW.assigned_to
    )::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_task_notify ON tasks;
CREATE TRIGGER trg_task_notify AFTER UPDATE ON tasks FOR EACH ROW EXECUTE FUNCTION notify_task_update();
