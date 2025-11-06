-- Drop indexes
DROP INDEX IF EXISTS "idx_play_history_file_id";
DROP INDEX IF EXISTS "idx_play_history_started";
DROP INDEX IF EXISTS "idx_video_queue_position";
DROP INDEX IF EXISTS "idx_video_queue_played";

-- Restore old play_history table
DROP TABLE IF EXISTS "play_history";
CREATE TABLE IF NOT EXISTS "play_history" (
      "play_time" INTEGER NOT NULL,
      "filename" VARCHAR(250) NOT NULL
);

-- Drop video_queue table
DROP TABLE IF EXISTS "video_queue";
