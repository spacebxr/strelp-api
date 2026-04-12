-- Create the presences table
CREATE TABLE IF NOT EXISTS presences (
    user_id TEXT PRIMARY KEY,
    data JSONB NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Trigger function to notify on changes
CREATE OR REPLACE FUNCTION notify_presence_change()
RETURNS TRIGGER AS $$
BEGIN
    -- NOTIFY channel name is 'presence_updates'
    -- Payload is the user_id
    PERFORM pg_notify('presence_updates', NEW.user_id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply trigger to the table
DROP TRIGGER IF EXISTS presence_change_trigger ON presences;
CREATE TRIGGER presence_change_trigger
AFTER INSERT OR UPDATE ON presences
FOR EACH ROW EXECUTE FUNCTION notify_presence_change();
