# WebSocket File Upload API

This document describes the WebSocket-based chunked file upload feature for the TV Streamer application.

## Overview

The upload feature allows clients to upload large video files via WebSocket using chunked transfer. After upload, files are automatically validated for format and dimensions, then stored in the database with an "inactive" status.

## Configuration

Configure upload settings in `config.yaml`:

```yaml
upload:
  upload_dir: "./uploads"              # Temporary upload directory
  max_file_size_mb: 5000               # Maximum file size in MB
  chunk_size_bytes: 262144             # Chunk size (256KB recommended)
  allowed_formats: ["mp4", "mkv", "avi", "mov", "webm"]
  required_width: 1920                 # Required video width
  required_height: 1080                # Required video height
```

## Upload Flow

1. **Initialize Upload**: Client sends upload initialization with filename and size
2. **Receive Session ID**: Server validates and returns a session ID
3. **Send Chunks**: Client sends file in chunks with base64 encoding
4. **Complete Upload**: Client signals completion after all chunks sent
5. **Validation**: Server validates format and dimensions using ffprobe
6. **Database Storage**: File metadata stored with `is_active = 0`

## WebSocket Messages

### 1. Upload Initialization

**Client → Server:**
```json
{
  "type": "upload_init",
  "filename": "video.mp4",
  "file_size": 104857600
}
```

**Server → Client (Success):**
```json
{
  "type": "upload_init_success",
  "success": true,
  "session_id": "abc123def456...",
  "message": "Upload session initialized. Ready to receive chunks."
}
```

**Server → Client (Error):**
```json
{
  "type": "upload_error",
  "success": false,
  "error": "File size exceeds maximum allowed size of 5000 MB"
}
```

### 2. Upload Chunk

**Client → Server:**
```json
{
  "type": "upload_chunk",
  "session_id": "abc123def456...",
  "chunk_data": "base64_encoded_chunk_data...",
  "chunk_num": 0
}
```

**Server → Client:**
```json
{
  "type": "upload_chunk_ack",
  "success": true,
  "session_id": "abc123def456...",
  "message": "Chunk 0 received"
}
```

### 3. Upload Complete

**Client → Server:**
```json
{
  "type": "upload_complete",
  "session_id": "abc123def456..."
}
```

**Server → Client (Success):**
```json
{
  "type": "upload_complete",
  "success": true,
  "file_id": "a1b2c3d4e5f6g7h8",
  "message": "File uploaded and validated successfully. File marked as inactive."
}
```

**Server → Client (Error):**
```json
{
  "type": "upload_error",
  "success": false,
  "error": "File validation failed: invalid video dimensions: expected 1920x1080, got 1280x720"
}
```

## Validation Process

After upload completion, the server:

1. **Verifies file size** matches expected total
2. **Runs ffprobe** to extract video metadata
3. **Validates format** against allowed formats
4. **Checks dimensions** match required width/height
5. **Moves file** from temp directory to video files directory
6. **Stores metadata** in database with inactive status

## Error Handling

Common errors:

- **File too large**: Exceeds `max_file_size_mb`
- **Invalid format**: File extension not in `allowed_formats`
- **Size mismatch**: Received bytes don't match declared size
- **Invalid dimensions**: Video resolution doesn't match requirements
- **No video stream**: File doesn't contain valid video stream
- **Database error**: Failed to store metadata

## Testing

Use the included `upload_test.html` file to test uploads:

1. Open `upload_test.html` in a web browser
2. Click "Connect WebSocket"
3. Select a video file
4. Click "Upload File"
4. Monitor progress and logs

## Example Client Code

```javascript
const ws = new WebSocket('ws://localhost:8080/api/ws');
const CHUNK_SIZE = 256 * 1024; // 256KB

// Initialize upload
ws.send(JSON.stringify({
  type: 'upload_init',
  filename: file.name,
  file_size: file.size
}));

// Send chunk (after receiving session_id)
const chunk = file.slice(start, end);
const reader = new FileReader();
reader.onload = (e) => {
  const base64Data = btoa(
    new Uint8Array(e.target.result)
      .reduce((data, byte) => data + String.fromCharCode(byte), '')
  );

  ws.send(JSON.stringify({
    type: 'upload_chunk',
    session_id: sessionId,
    chunk_data: base64Data,
    chunk_num: chunkNumber
  }));
};
reader.readAsArrayBuffer(chunk);

// Complete upload
ws.send(JSON.stringify({
  type: 'upload_complete',
  session_id: sessionId
}));
```

## Database Schema

Files are stored in the `availible_files` table:

```sql
CREATE TABLE "availible_files" (
  "file_id" VARCHAR(50) NOT NULL,
  "filepath" VARCHAR(250) NOT NULL,
  "file_size" INTEGER NOT NULL,
  "video_length" INTEGER NOT NULL,
  "added_time" INTEGER NOT NULL,
  "ffprobe_data" TEXT NULL DEFAULT '{}',
  "is_active" INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY ("file_id")
);
```

- `is_active = 0`: File uploaded but not activated
- `is_active = 1`: File activated and available for streaming

## Activating Files

To activate an uploaded file, update the database:

```sql
UPDATE availible_files SET is_active = 1 WHERE file_id = 'your_file_id';
```

## Security Considerations

- File size validation prevents DoS attacks
- Format validation prevents malicious file execution
- Dimension validation ensures quality standards
- Files are inactive by default requiring manual activation
- Temporary files are cleaned up on error
- Session IDs are unique and cryptographically generated

## Requirements

- **ffprobe**: Must be installed and available in system PATH
- **WebSocket**: Client must support WebSocket protocol
- **Disk space**: Ensure sufficient space in upload and video directories
