-- Create schedule table for managing playback order in endless loop
CREATE TABLE IF NOT EXISTS "schedule" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "file_id" VARCHAR(50) NOT NULL,
    "filepath" VARCHAR(250) NOT NULL,
    "schedule_position" INTEGER NOT NULL,
    "is_current" INTEGER NOT NULL DEFAULT 0,
    "added_at" INTEGER NOT NULL,
    FOREIGN KEY ("file_id") REFERENCES "availible_files"("file_id") ON DELETE CASCADE
);

-- Create index for efficient lookups
CREATE INDEX IF NOT EXISTS "idx_schedule_position" ON "schedule"("schedule_position");
CREATE INDEX IF NOT EXISTS "idx_schedule_current" ON "schedule"("is_current");
