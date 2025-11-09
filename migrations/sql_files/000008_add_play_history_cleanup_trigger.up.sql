-- Add trigger to automatically cleanup play_history records older than 1 day
-- This trigger runs after each insert and removes old records efficiently

CREATE TRIGGER IF NOT EXISTS cleanup_old_play_history
AFTER INSERT ON play_history
BEGIN
    -- Delete records older than 1 day (86400 seconds)
DELETE FROM play_history
WHERE started_at < (unixepoch() - 86400);

END;