DROP TABLE IF EXISTS "availible_files";
CREATE TABLE IF NOT EXISTS "availible_files" (
     "file_id" VARCHAR(50) NOT NULL,
     "filepath" VARCHAR(250) NOT NULL,
     "file_size" INTEGER NOT NULL,
     "video_length" INTEGER NOT NULL,
     "added_time" INTEGER NOT NULL,
     "ffprobe_data" TEXT NULL DEFAULT '{}',
     PRIMARY KEY ("file_id")
);

DROP TABLE IF EXISTS "play_history";
CREATE TABLE IF NOT EXISTS "play_history" (
      "play_time" INTEGER NOT NULL,
      "filename" VARCHAR(250) NOT NULL
);
