-- Rollback: Add back the filepath columns

-- Step 1: Recreate video_queue with filepath column
CREATE TABLE IF NOT EXISTS "video_queue_new" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "file_id" VARCHAR(50) NOT NULL,
    "filepath" VARCHAR(250) NOT NULL DEFAULT '',
    "added_at" INTEGER NOT NULL,
    "played" INTEGER NOT NULL DEFAULT 0,
    "played_at" INTEGER NULL,
    "queue_position" INTEGER NOT NULL DEFAULT 0,
    "is_ad" INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY ("file_id") REFERENCES "availible_files"("file_id") ON DELETE CASCADE
);

-- Copy data back
INSERT INTO "video_queue_new" ("id", "file_id", "filepath", "added_at", "played", "played_at", "queue_position", "is_ad")
SELECT vq."id", vq."file_id", COALESCE(af."filepath", ''), vq."added_at", vq."played", vq."played_at", vq."queue_position", vq."is_ad"
FROM "video_queue" vq
LEFT JOIN "availible_files" af ON vq."file_id" = af."file_id";

DROP TABLE "video_queue";
ALTER TABLE "video_queue_new" RENAME TO "video_queue";

CREATE INDEX IF NOT EXISTS "idx_video_queue_played" ON "video_queue"("played");
CREATE INDEX IF NOT EXISTS "idx_video_queue_position" ON "video_queue"("queue_position");

-- Step 2: Recreate schedule with filepath column
CREATE TABLE IF NOT EXISTS "schedule_new" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "file_id" VARCHAR(50) NOT NULL,
    "filepath" VARCHAR(250) NOT NULL DEFAULT '',
    "schedule_position" INTEGER NOT NULL,
    "is_current" INTEGER NOT NULL DEFAULT 0,
    "added_at" INTEGER NOT NULL,
    FOREIGN KEY ("file_id") REFERENCES "availible_files"("file_id") ON DELETE CASCADE
);

-- Copy data back
INSERT INTO "schedule_new" ("id", "file_id", "filepath", "schedule_position", "is_current", "added_at")
SELECT s."id", s."file_id", COALESCE(af."filepath", ''), s."schedule_position", s."is_current", s."added_at"
FROM "schedule" s
LEFT JOIN "availible_files" af ON s."file_id" = af."file_id";

DROP TABLE "schedule";
ALTER TABLE "schedule_new" RENAME TO "schedule";

CREATE INDEX IF NOT EXISTS "idx_schedule_position" ON "schedule"("schedule_position");
CREATE INDEX IF NOT EXISTS "idx_schedule_current" ON "schedule"("is_current");

-- Step 3: Recreate play_history with filename and filepath columns
CREATE TABLE IF NOT EXISTS "play_history_new" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "file_id" VARCHAR(50) NULL,
    "filename" VARCHAR(250) NOT NULL DEFAULT '',
    "filepath" VARCHAR(250) NOT NULL DEFAULT '',
    "started_at" INTEGER NOT NULL,
    "finished_at" INTEGER NULL,
    "duration_seconds" INTEGER NULL,
    "is_ad" INTEGER NOT NULL DEFAULT 0,
    "skip_requested" INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY ("file_id") REFERENCES "availible_files"("file_id") ON DELETE SET NULL
);

-- Copy data back
INSERT INTO "play_history_new" ("id", "file_id", "filename", "filepath", "started_at", "finished_at", "duration_seconds", "is_ad", "skip_requested")
SELECT ph."id", ph."file_id", '', COALESCE(af."filepath", ''), ph."started_at", ph."finished_at", ph."duration_seconds", ph."is_ad", ph."skip_requested"
FROM "play_history" ph
LEFT JOIN "availible_files" af ON ph."file_id" = af."file_id";

DROP TABLE "play_history";
ALTER TABLE "play_history_new" RENAME TO "play_history";

CREATE INDEX IF NOT EXISTS "idx_play_history_started" ON "play_history"("started_at");
CREATE INDEX IF NOT EXISTS "idx_play_history_file_id" ON "play_history"("file_id");
