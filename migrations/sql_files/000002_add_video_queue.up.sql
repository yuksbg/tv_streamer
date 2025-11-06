-- Create video_queue table for managing the streaming queue
CREATE TABLE IF NOT EXISTS "video_queue" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "file_id" VARCHAR(50) NOT NULL,
    "filepath" VARCHAR(250) NOT NULL,
    "added_at" INTEGER NOT NULL,
    "played" INTEGER NOT NULL DEFAULT 0,
    "played_at" INTEGER NULL,
    "queue_position" INTEGER NOT NULL DEFAULT 0,
    "is_ad" INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY ("file_id") REFERENCES "availible_files"("file_id") ON DELETE CASCADE
);

-- Create index for faster queue lookups
CREATE INDEX IF NOT EXISTS "idx_video_queue_played" ON "video_queue"("played");
CREATE INDEX IF NOT EXISTS "idx_video_queue_position" ON "video_queue"("queue_position");

-- Update play_history table to include more metadata
DROP TABLE IF EXISTS "play_history";
CREATE TABLE IF NOT EXISTS "play_history" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "file_id" VARCHAR(50) NOT NULL,
    "filename" VARCHAR(250) NOT NULL,
    "filepath" VARCHAR(250) NOT NULL,
    "started_at" INTEGER NOT NULL,
    "finished_at" INTEGER NULL,
    "duration_seconds" INTEGER NULL,
    "is_ad" INTEGER NOT NULL DEFAULT 0,
    "skip_requested" INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY ("file_id") REFERENCES "availible_files"("file_id") ON DELETE SET NULL
);

-- Create index for faster history queries
CREATE INDEX IF NOT EXISTS "idx_play_history_started" ON "play_history"("started_at");
CREATE INDEX IF NOT EXISTS "idx_play_history_file_id" ON "play_history"("file_id");
