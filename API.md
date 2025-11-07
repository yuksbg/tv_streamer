# TV Streamer API Documentation

## Table of Contents

- [REST API](#rest-api)
  - [Health Check](#health-check)
  - [Stream Control](#stream-control)
  - [Schedule Management](#schedule-management)
- [WebSocket API](#websocket-api)
  - [Connection](#connection)
  - [Message Types](#message-types)
  - [Usage Examples](#usage-examples)

---

## REST API

Base URL: `http://localhost:8080/api`

### Health Check

#### GET `/health`

Check if the service is running.

**Response:**
```json
{
  "status": true,
  "service": "tv_streamer",
  "version": "1.0.0"
}
```

---

### Stream Control

#### POST `/stream/next`

Skip to the next video in the queue.

**Response:**
```json
{
  "success": true,
  "message": "Skipped to next video"
}
```

---

#### POST `/stream/add?file={filepath}`

Add a video to the queue.

**Query Parameters:**
- `file` (required): Full path to the video file

**Example:**
```bash
curl -X POST "http://localhost:8080/api/stream/add?file=/path/to/video.ts"
```

**Response:**
```json
{
  "success": true,
  "message": "Video added to queue",
  "file": "/path/to/video.ts"
}
```

---

#### GET `/stream/queue`

Get the current video queue.

**Response:**
```json
{
  "success": true,
  "count": 5,
  "queue": [
    {
      "id": 1,
      "file_id": "abc123def456",
      "filepath": "/path/to/video1.ts",
      "added_at": 1699286400,
      "played": 0,
      "queue_position": 0,
      "is_ad": 0
    }
  ]
}
```

---

#### GET `/stream/status`

Get the current player status.

**Response:**
```json
{
  "success": true,
  "status": {
    "running": true,
    "ffmpeg_running": true,
    "current_video": {
      "file_id": "abc123def456",
      "filepath": "/path/to/video.ts",
      "is_ad": false
    },
    "playback_started_at": "2025-11-07T12:34:56Z",
    "playback_duration_seconds": 125
  }
}
```

---

#### POST `/stream/inject-ad?file={filepath}`

Inject an advertisement at the front of the queue.

**Query Parameters:**
- `file` (required): Full path to the ad video file

**Example:**
```bash
curl -X POST "http://localhost:8080/api/stream/inject-ad?file=/path/to/ad.ts"
```

**Response:**
```json
{
  "success": true,
  "message": "Ad injected successfully",
  "file": "/path/to/ad.ts"
}
```

---

#### GET `/stream/history?limit={limit}`

Get play history.

**Query Parameters:**
- `limit` (optional): Number of records to return (default: 50)

**Example:**
```bash
curl "http://localhost:8080/api/stream/history?limit=20"
```

**Response:**
```json
{
  "success": true,
  "count": 20,
  "history": [
    {
      "id": 1,
      "file_id": "abc123def456",
      "filename": "video.ts",
      "filepath": "/path/to/video.ts",
      "started_at": 1699286400,
      "finished_at": 1699287000,
      "duration_seconds": 600,
      "is_ad": 0,
      "skip_requested": 0
    }
  ]
}
```

---

#### POST `/stream/scan?directory={path}`

Scan a directory for videos and add them to the library.

**Query Parameters:**
- `directory` (required): Path to the directory containing video files

**Example:**
```bash
curl -X POST "http://localhost:8080/api/stream/scan?directory=/path/to/videos"
```

**Response:**
```json
{
  "success": true,
  "message": "Directory scanned successfully",
  "videos_added": 15,
  "directory": "/path/to/videos"
}
```

---

#### POST `/stream/clear-played`

Clear all played items from the queue.

**Response:**
```json
{
  "success": true,
  "message": "Played items cleared from queue",
  "deleted_count": 10
}
```

---

### Schedule Management

#### POST `/schedule/add?file={filepath}`

Add a video to the endless loop schedule.

**Query Parameters:**
- `file` (required): Full path to the video file

**Response:**
```json
{
  "success": true,
  "message": "Video added to schedule",
  "file": "/path/to/video.ts"
}
```

---

#### GET `/schedule/`

Get the current schedule.

**Response:**
```json
{
  "success": true,
  "count": 10,
  "schedule": [
    {
      "id": 1,
      "file_id": "abc123def456",
      "filepath": "/path/to/video.ts",
      "schedule_position": 0,
      "is_current": 0,
      "added_at": 1699286400
    }
  ]
}
```

---

#### DELETE `/schedule/remove?file_id={file_id}`

Remove a video from the schedule.

**Query Parameters:**
- `file_id` (required): File ID of the video to remove

**Response:**
```json
{
  "success": true,
  "message": "Video removed from schedule",
  "file_id": "abc123def456"
}
```

---

#### POST `/schedule/clear`

Clear all items from the schedule.

**Response:**
```json
{
  "success": true,
  "message": "Schedule cleared",
  "deleted_count": 10
}
```

---

#### POST `/schedule/reset`

Reset the schedule position to the beginning.

**Response:**
```json
{
  "success": true,
  "message": "Schedule position reset to beginning"
}
```

---

## WebSocket API

### Connection

**Endpoint:** `ws://localhost:8080/api/ws`

**Protocol:** WebSocket (RFC 6455)

**Connection Example:**
```javascript
const ws = new WebSocket('ws://localhost:8080/api/ws');

ws.onopen = () => {
  console.log('Connected to TV Streamer WebSocket');
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received:', data);
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Disconnected from TV Streamer WebSocket');
};
```

---

### Message Types

The WebSocket API sends three types of messages, all in JSON format:

#### 1. Connection Status

Sent immediately upon successful connection.

**Format:**
```json
{
  "type": "connection",
  "status": "connected",
  "message": "Connected to TV Streamer WebSocket API"
}
```

**Fields:**
- `type` (string): Always "connection"
- `status` (string): Connection status ("connected")
- `message` (string): Human-readable status message

---

#### 2. Logs (Structured)

Real-time application logs broadcast to all connected clients.

**Format:**
```json
{
  "type": "logs",
  "level": "info",
  "message": "Video playback started",
  "timestamp": "2025-11-07T12:34:56.789Z",
  "fields": {
    "module": "streamer",
    "file_id": "abc123def456",
    "filepath": "/videos/movie.ts",
    "video_id": 1
  }
}
```

**Fields:**
- `type` (string): Always "logs"
- `level` (string): Log level - one of: "debug", "info", "warn", "error", "fatal", "panic"
- `message` (string): The log message text
- `timestamp` (string): ISO 8601 formatted timestamp with milliseconds
- `fields` (object, optional): Additional context fields from the logger

**Log Levels:**
- `debug`: Detailed debugging information
- `info`: General informational messages
- `warn`: Warning messages
- `error`: Error messages
- `fatal`: Fatal errors (application will exit)
- `panic`: Panic messages (application crashed)

**Common Fields:**
- `module`: Module name (e.g., "streamer", "web", "database")
- `file_id`: MD5 hash of the file path
- `filepath`: Full path to the video file
- `video_id`: Video queue ID
- `history_id`: Play history record ID
- `handler`: HTTP handler name
- `client_ip`: Client IP address

---

#### 3. Currently Playing

Broadcast when a new video starts playing.

**Format:**
```json
{
  "type": "currently_playing",
  "file_id": "abc123def456",
  "started_time": 1699286400
}
```

**Fields:**
- `type` (string): Always "currently_playing"
- `file_id` (string): MD5 hash of the video file path
- `started_time` (integer): Unix timestamp when playback started

---

### Usage Examples

#### Basic Connection and Message Handling

```javascript
const ws = new WebSocket('ws://localhost:8080/api/ws');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);

  switch (data.type) {
    case 'connection':
      console.log(`✓ ${data.message}`);
      break;

    case 'logs':
      handleLog(data);
      break;

    case 'currently_playing':
      handleNowPlaying(data);
      break;

    default:
      console.log('Unknown message type:', data.type);
  }
};

function handleLog(log) {
  const timestamp = new Date(log.timestamp).toLocaleTimeString();
  const level = log.level.toUpperCase().padEnd(5);
  const fields = log.fields ? ` ${JSON.stringify(log.fields)}` : '';

  console.log(`[${timestamp}] [${level}] ${log.message}${fields}`);
}

function handleNowPlaying(data) {
  const startDate = new Date(data.started_time * 1000);
  console.log(`🎬 Now Playing: ${data.file_id}`);
  console.log(`   Started at: ${startDate.toLocaleString()}`);
}
```

---

#### Filter Logs by Level

```javascript
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);

  if (data.type === 'logs') {
    // Only show errors and warnings
    if (data.level === 'error' || data.level === 'warn') {
      console.error(`[${data.level}] ${data.message}`, data.fields);
    }
  }
};
```

---

#### Filter Logs by Module

```javascript
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);

  if (data.type === 'logs' && data.fields?.module === 'streamer') {
    // Only show logs from the streamer module
    console.log(`[STREAMER] ${data.message}`);
  }
};
```

---

#### Display Currently Playing with Duration

```javascript
let currentVideo = null;
let startTime = null;

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);

  if (data.type === 'currently_playing') {
    currentVideo = data.file_id;
    startTime = data.started_time;

    updateNowPlaying();
  }
};

function updateNowPlaying() {
  if (!currentVideo || !startTime) return;

  const elapsed = Math.floor(Date.now() / 1000) - startTime;
  const minutes = Math.floor(elapsed / 60);
  const seconds = elapsed % 60;

  console.log(`Now Playing: ${currentVideo} (${minutes}:${seconds.toString().padStart(2, '0')})`);

  // Update every second
  setTimeout(updateNowPlaying, 1000);
}
```

---

#### React Hook Example

```javascript
import { useEffect, useState } from 'react';

function useWebSocketLogs(url) {
  const [logs, setLogs] = useState([]);
  const [currentVideo, setCurrentVideo] = useState(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    const ws = new WebSocket(url);

    ws.onopen = () => {
      setConnected(true);
    };

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);

      switch (data.type) {
        case 'logs':
          setLogs(prev => [...prev, data].slice(-100)); // Keep last 100 logs
          break;

        case 'currently_playing':
          setCurrentVideo({
            fileId: data.file_id,
            startedTime: data.started_time
          });
          break;
      }
    };

    ws.onclose = () => {
      setConnected(false);
    };

    return () => {
      ws.close();
    };
  }, [url]);

  return { logs, currentVideo, connected };
}

// Usage in component
function StreamMonitor() {
  const { logs, currentVideo, connected } = useWebSocketLogs('ws://localhost:8080/api/ws');

  return (
    <div>
      <h2>Connection Status: {connected ? '✓ Connected' : '✗ Disconnected'}</h2>

      {currentVideo && (
        <div>
          <h3>Now Playing</h3>
          <p>File ID: {currentVideo.fileId}</p>
          <p>Started: {new Date(currentVideo.startedTime * 1000).toLocaleString()}</p>
        </div>
      )}

      <div>
        <h3>Logs</h3>
        {logs.map((log, index) => (
          <div key={index} className={`log-${log.level}`}>
            [{log.timestamp}] [{log.level}] {log.message}
          </div>
        ))}
      </div>
    </div>
  );
}
```

---

#### Python Client Example

```python
import websocket
import json
import threading

def on_message(ws, message):
    data = json.loads(message)

    if data['type'] == 'connection':
        print(f"✓ {data['message']}")

    elif data['type'] == 'logs':
        level = data['level'].upper()
        timestamp = data['timestamp']
        message = data['message']
        fields = data.get('fields', {})

        print(f"[{timestamp}] [{level}] {message}")
        if fields:
            print(f"  Fields: {fields}")

    elif data['type'] == 'currently_playing':
        print(f"🎬 Now Playing: {data['file_id']}")
        print(f"   Started: {data['started_time']}")

def on_error(ws, error):
    print(f"Error: {error}")

def on_close(ws, close_status_code, close_msg):
    print("Connection closed")

def on_open(ws):
    print("Connected to TV Streamer WebSocket")

if __name__ == "__main__":
    ws = websocket.WebSocketApp(
        "ws://localhost:8080/api/ws",
        on_open=on_open,
        on_message=on_message,
        on_error=on_error,
        on_close=on_close
    )

    ws.run_forever()
```

---

## HLS Stream

The HLS stream is available at:

**URL:** `http://localhost:8080/stream/stream.m3u8`

### Playing with VLC

```bash
vlc http://localhost:8080/stream/stream.m3u8
```

### Playing with FFplay

```bash
ffplay http://localhost:8080/stream/stream.m3u8
```

### Playing in Browser (hls.js)

```html
<!DOCTYPE html>
<html>
<head>
  <title>TV Streamer</title>
</head>
<body>
  <video id="video" controls width="1280" height="720"></video>
  <script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
  <script>
    const video = document.getElementById('video');
    if (Hls.isSupported()) {
      const hls = new Hls();
      hls.loadSource('http://localhost:8080/stream/stream.m3u8');
      hls.attachMedia(video);
      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        video.play();
      });
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      // Native HLS support (Safari)
      video.src = 'http://localhost:8080/stream/stream.m3u8';
    }
  </script>
</body>
</html>
```

---

## Error Handling

All REST API endpoints return errors in the following format:

```json
{
  "success": false,
  "error": "Error message describing what went wrong"
}
```

Common HTTP status codes:
- `200 OK`: Request successful
- `400 Bad Request`: Missing or invalid parameters
- `500 Internal Server Error`: Server-side error

---

## Rate Limiting

Currently, there is no rate limiting implemented. For production use, consider adding a reverse proxy (nginx, Caddy) with rate limiting enabled.

---

## CORS

CORS is enabled for all origins (`*`). For production use, configure specific allowed origins in `config.yaml` or through environment variables.

---

## WebSocket Connection Limits

The WebSocket hub supports unlimited concurrent connections. Each connection:
- Has a 256-message broadcast buffer
- Automatically removes disconnected clients
- Supports ping/pong for keep-alive

If the broadcast buffer is full, new messages will be dropped with a warning logged.
