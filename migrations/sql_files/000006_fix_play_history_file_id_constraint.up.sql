-- Fix the play_history table to allow NULL in file_id column
-- This is necessary for the ON DELETE SET NULL foreign key constraint to work
-- SQLite doesn't support ALTER COLUMN, so we need to recreate the table

-- Create a temporary table with the correct schema
CREATE TABLE IF NOT EXISTS "play_history_new" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "file_id" VARCHAR(50) NULL,
    "filename" VARCHAR(250) NOT NULL,
    "filepath" VARCHAR(250) NOT NULL,
    "started_at" INTEGER NOT NULL,
    "finished_at" INTEGER NULL,
    "duration_seconds" INTEGER NULL,
    "is_ad" INTEGER NOT NULL DEFAULT 0,
    "skip_requested" INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY ("file_id") REFERENCES "availible_files"("file_id") ON DELETE SET NULL
);

-- Copy data from old table to new table
INSERT INTO "play_history_new"
SELECT * FROM "play_history";

-- Drop the old table
DROP TABLE "play_history";

-- Rename the new table to the original name
ALTER TABLE "play_history_new" RENAME TO "play_history";

-- Recreate indexes
CREATE INDEX IF NOT EXISTS "idx_play_history_started" ON "play_history"("started_at");
CREATE INDEX IF NOT EXISTS "idx_play_history_file_id" ON "play_history"("file_id");
