# Testing File Description Feature

## Overview
This document describes how to test the new file description feature that was added to the TV Streamer application.

## Changes Made

### 1. Database Schema
- Added `description` column to `availible_files` table
- Migration files created:
  - `000005_add_description_to_available_files.up.sql`
  - `000005_add_description_to_available_files.down.sql`

### 2. Model Updates
- Updated `AvailableFiles` struct in `modules/streamer/models/available_files.go`
- Added `Description` field: `string` with max length 500 characters

### 3. Business Logic
- Added `UpdateFileDescription()` function in `modules/streamer/available_files.go`

### 4. REST API
- Added new endpoint: `PUT /api/files/:file_id/description`
- Handler: `handleFileUpdateDescription` in `modules/web/file_handlers.go`

## Testing Steps

### 1. Run Migration
```bash
# Start the application - it should automatically run the migration
./tv_streamer
```

### 2. Verify Database Schema
```bash
# Check if the description column was added
sqlite3 tv_streamer.db "PRAGMA table_info(availible_files);"
```

Expected output should include:
```
description|VARCHAR(500)|0||''
```

### 3. Test API Endpoints

#### a. List All Files (Check Description Field)
```bash
curl -X GET http://localhost:8080/api/files/
```

Expected response:
```json
{
  "success": true,
  "files": [
    {
      "file_id": "...",
      "filepath": "...",
      "file_size": ...,
      "video_length": ...,
      "added_time": ...,
      "ffprobe_data": "...",
      "is_active": 0,
      "description": ""
    }
  ],
  "count": ...
}
```

#### b. Get Single File Info
```bash
curl -X GET http://localhost:8080/api/files/{file_id}
```

Expected response includes `description` field.

#### c. Update File Description
```bash
curl -X PUT http://localhost:8080/api/files/{file_id}/description \
  -H "Content-Type: application/json" \
  -d '{"description": "This is a test video file"}'
```

Expected response:
```json
{
  "success": true,
  "message": "File description updated successfully",
  "file_id": "...",
  "description": "This is a test video file"
}
```

#### d. Verify Description Was Updated
```bash
curl -X GET http://localhost:8080/api/files/{file_id}
```

The response should now show the updated description.

#### e. Clear Description
```bash
curl -X PUT http://localhost:8080/api/files/{file_id}/description \
  -H "Content-Type: application/json" \
  -d '{"description": ""}'
```

#### f. Test Long Description (Max 500 characters)
```bash
curl -X PUT http://localhost:8080/api/files/{file_id}/description \
  -H "Content-Type: application/json" \
  -d '{"description": "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium."}'
```

### 4. Error Cases to Test

#### a. Non-existent File ID
```bash
curl -X PUT http://localhost:8080/api/files/invalid_id/description \
  -H "Content-Type: application/json" \
  -d '{"description": "Test"}'
```

Expected response:
```json
{
  "success": false,
  "error": "File not found"
}
```

#### b. Missing Description Field
```bash
curl -X PUT http://localhost:8080/api/files/{file_id}/description \
  -H "Content-Type: application/json" \
  -d '{}'
```

Expected response:
```json
{
  "success": false,
  "error": "Invalid request body: description field is required"
}
```

#### c. Invalid JSON
```bash
curl -X PUT http://localhost:8080/api/files/{file_id}/description \
  -H "Content-Type: application/json" \
  -d 'invalid json'
```

Expected response:
```json
{
  "success": false,
  "error": "Invalid request body: description field is required"
}
```

## Integration Testing

1. Add a file to the system
2. Set a description for the file
3. Add the file to the queue or schedule
4. Verify the description persists
5. Rename the file and verify description is maintained
6. List all files and verify descriptions are shown

## Rollback Testing

To test the down migration:
```bash
# Manually run the down migration
sqlite3 tv_streamer.db < migrations/sql_files/000005_add_description_to_available_files.down.sql

# Verify the column is removed
sqlite3 tv_streamer.db "PRAGMA table_info(availible_files);"
```

The `description` column should no longer be present.

## Expected Log Messages

When updating a description, you should see log messages like:
```
✓ File description updated successfully
```

## Notes

- The description field is optional and defaults to an empty string
- Maximum length is 500 characters
- The field is nullable in the database
- All existing files will have an empty description by default after migration
