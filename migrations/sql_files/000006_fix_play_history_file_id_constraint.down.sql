-- Revert the play_history table back to NOT NULL file_id
-- SQLite doesn't support ALTER COLUMN, so we need to recreate the table

-- Create a temporary table with the old schema
CREATE TABLE IF NOT EXISTS "play_history_new" (
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

-- Copy data from old table to new table (excluding NULL file_ids)
INSERT INTO "play_history_new"
SELECT * FROM "play_history" WHERE "file_id" IS NOT NULL;

-- Drop the old table
DROP TABLE "play_history";

-- Rename the new table to the original name
ALTER TABLE "play_history_new" RENAME TO "play_history";

-- Recreate indexes
CREATE INDEX IF NOT EXISTS "idx_play_history_started" ON "play_history"("started_at");
CREATE INDEX IF NOT EXISTS "idx_play_history_file_id" ON "play_history"("file_id");
