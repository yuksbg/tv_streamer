-- Remove redundant filepath columns from tables
-- Make available_files the single source of truth for file paths

-- Step 1: Recreate video_queue without filepath column
CREATE TABLE IF NOT EXISTS "video_queue_new" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "file_id" VARCHAR(50) NOT NULL,
    "added_at" INTEGER NOT NULL,
    "played" INTEGER NOT NULL DEFAULT 0,
    "played_at" INTEGER NULL,
    "queue_position" INTEGER NOT NULL DEFAULT 0,
    "is_ad" INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY ("file_id") REFERENCES "availible_files"("file_id") ON DELETE CASCADE
);

-- Copy data from old video_queue to new (excluding filepath)
INSERT INTO "video_queue_new" ("id", "file_id", "added_at", "played", "played_at", "queue_position", "is_ad")
SELECT "id", "file_id", "added_at", "played", "played_at", "queue_position", "is_ad"
FROM "video_queue";

-- Drop old table and rename new one
DROP TABLE "video_queue";
ALTER TABLE "video_queue_new" RENAME TO "video_queue";

-- Recreate indexes
CREATE INDEX IF NOT EXISTS "idx_video_queue_played" ON "video_queue"("played");
CREATE INDEX IF NOT EXISTS "idx_video_queue_position" ON "video_queue"("queue_position");

-- Step 2: Recreate schedule without filepath column
CREATE TABLE IF NOT EXISTS "schedule_new" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "file_id" VARCHAR(50) NOT NULL,
    "schedule_position" INTEGER NOT NULL,
    "is_current" INTEGER NOT NULL DEFAULT 0,
    "added_at" INTEGER NOT NULL,
    FOREIGN KEY ("file_id") REFERENCES "availible_files"("file_id") ON DELETE CASCADE
);

-- Copy data from old schedule to new (excluding filepath)
INSERT INTO "schedule_new" ("id", "file_id", "schedule_position", "is_current", "added_at")
SELECT "id", "file_id", "schedule_position", "is_current", "added_at"
FROM "schedule";

-- Drop old table and rename new one
DROP TABLE "schedule";
ALTER TABLE "schedule_new" RENAME TO "schedule";

-- Recreate indexes
CREATE INDEX IF NOT EXISTS "idx_schedule_position" ON "schedule"("schedule_position");
CREATE INDEX IF NOT EXISTS "idx_schedule_current" ON "schedule"("is_current");

-- Step 3: Recreate play_history without filename and filepath columns
-- Also change ON DELETE SET NULL to ON DELETE CASCADE (user wants to remove history when file is deleted)
CREATE TABLE IF NOT EXISTS "play_history_new" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "file_id" VARCHAR(50) NOT NULL,
    "started_at" INTEGER NOT NULL,
    "finished_at" INTEGER NULL,
    "duration_seconds" INTEGER NULL,
    "is_ad" INTEGER NOT NULL DEFAULT 0,
    "skip_requested" INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY ("file_id") REFERENCES "availible_files"("file_id") ON DELETE CASCADE
);

-- Copy data from old play_history to new (excluding filename and filepath)
INSERT INTO "play_history_new" ("id", "file_id", "started_at", "finished_at", "duration_seconds", "is_ad", "skip_requested")
SELECT "id", "file_id", "started_at", "finished_at", "duration_seconds", "is_ad", "skip_requested"
FROM "play_history"
WHERE "file_id" IS NOT NULL;

-- Drop old table and rename new one
DROP TABLE "play_history";
ALTER TABLE "play_history_new" RENAME TO "play_history";

-- Recreate indexes
CREATE INDEX IF NOT EXISTS "idx_play_history_started" ON "play_history"("started_at");
CREATE INDEX IF NOT EXISTS "idx_play_history_file_id" ON "play_history"("file_id");
