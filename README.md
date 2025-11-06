# TV Streamer 📺

A lightweight, dynamic TV-style streaming platform built with Go and FFmpeg. Stream pre-encoded video files as a continuous HLS stream with real-time control via REST API.

## ✨ Features

- **Persistent FFmpeg Pipeline**: Seamless, continuous playback without gaps
- **Real-time HLS Output**: Compatible with browsers, VLC, Apple TV, and other HLS-capable players
- **REST API Control**: Skip files, enqueue content, inject ads on demand
- **SQLite3 Database**: Track play history, timestamps, and queue state
- **Detailed Logging**: Comprehensive logging at every step for monitoring and debugging
- **Queue Management**: Advanced queue system with position tracking
- **Ad Injection**: Inject ads dynamically into the stream
- **Play History**: Track what was played, when, and for how long

## 🏗️ Architecture

### How It Works

1. **FFmpeg** runs persistently with `-re -f mpegts -i pipe:0`, reading from Go's stdin
2. **Go** streams video files sequentially to FFmpeg's stdin in real-time
3. **FFmpeg** converts the stream to HLS format (`.m3u8` playlist + `.ts` segments)
4. **HTTP Server** serves HLS files and provides API endpoints
5. **SQLite Database** tracks what was played, when, and for how long

```
┌─────────────────┐
│  Video Files    │
│  (.ts, .mp4)    │
└────────┬────────┘
         │
         v
┌─────────────────┐      ┌──────────────┐      ┌─────────────┐
│   Go Streamer   │─────>│   FFmpeg     │─────>│ HLS Output  │
│   (File Queue)  │ pipe │   Pipeline   │      │ (.m3u8)     │
└─────────────────┘      └──────────────┘      └─────────────┘
         │                                               │
         v                                               v
┌─────────────────┐                            ┌─────────────┐
│ SQLite Database │                            │ HTTP Server │
│ (History/Queue) │                            │ (REST API)  │
└─────────────────┘                            └─────────────┘
```

## 📋 Requirements

- **Go 1.25+**
- **FFmpeg** with `libx264` and `aac` codec support
- Video files in supported formats (`.ts`, `.mp4`, `.mkv`, `.avi`, `.mov`)

## 🚀 Getting Started

### Installation

1. **Clone the repository**
   ```bash
   git clone <repository_url>
   cd tv_streamer
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Verify FFmpeg installation**
   ```bash
   ffmpeg -version
   ```

4. **Create video directory**
   ```bash
   mkdir -p videos
   ```

5. **Configure the application**
   Edit `config.yaml` to set your video files path and other settings:
   ```yaml
   app:
     web_port: 8080
     video_files_path: "./videos"
   database:
     db_path: "./"
   streaming:
     output_dir: "./out"
     hls_segment_time: 6
     hls_list_size: 10
     ffmpeg_preset: "veryfast"
     video_bitrate: "2000k"
     audio_bitrate: "128k"
   ```

### Running the Application

```bash
go run main.go
```

The application will:
- ✓ Check for FFmpeg installation
- ✓ Initialize SQLite database with migrations
- ✓ Start FFmpeg streaming pipeline
- ✓ Scan video directory and add files to queue
- ✓ Start HTTP server on port 8080

## 📡 API Endpoints

### Health Check
```bash
GET /api/health
```

### Stream Control

#### Get Player Status
```bash
GET /api/stream/status

Response:
{
  "success": true,
  "status": {
    "running": true,
    "current_video": {
      "file_id": "abc123",
      "filepath": "/path/to/video.mp4",
      "is_ad": false
    },
    "playback_started_at": "2025-11-06T12:00:00Z",
    "playback_duration_seconds": 45
  }
}
```

#### Skip to Next Video
```bash
POST /api/stream/next

Response:
{
  "success": true,
  "message": "Skipped to next video"
}
```

#### Add Video to Queue
```bash
POST /api/stream/add?file=/path/to/video.mp4

Response:
{
  "success": true,
  "message": "Video added to queue",
  "file": "/path/to/video.mp4"
}
```

#### Get Current Queue
```bash
GET /api/stream/queue

Response:
{
  "success": true,
  "count": 5,
  "queue": [
    {
      "id": 1,
      "file_id": "abc123",
      "filepath": "/path/to/video1.mp4",
      "added_at": 1699286400,
      "played": 0,
      "queue_position": 0,
      "is_ad": 0
    },
    ...
  ]
}
```

#### Inject Ad
```bash
POST /api/stream/inject-ad?file=/path/to/ad.mp4

Response:
{
  "success": true,
  "message": "Ad injected successfully",
  "file": "/path/to/ad.mp4"
}
```

#### Get Play History
```bash
GET /api/stream/history?limit=50

Response:
{
  "success": true,
  "count": 10,
  "history": [
    {
      "id": 1,
      "file_id": "abc123",
      "filename": "video.mp4",
      "filepath": "/path/to/video.mp4",
      "started_at": 1699286400,
      "finished_at": 1699287000,
      "duration_seconds": 600,
      "is_ad": 0,
      "skip_requested": 0
    },
    ...
  ]
}
```

#### Scan Directory for Videos
```bash
POST /api/stream/scan?directory=/path/to/videos

Response:
{
  "success": true,
  "message": "Directory scanned successfully",
  "videos_added": 15,
  "directory": "/path/to/videos"
}
```

#### Clear Played Items from Queue
```bash
POST /api/stream/clear-played

Response:
{
  "success": true,
  "message": "Played items cleared from queue",
  "deleted_count": 10
}
```

## 📺 Streaming

### HLS Stream URL
```
http://localhost:8080/stream/stream.m3u8
```

### Playing the Stream

#### VLC
```bash
vlc http://localhost:8080/stream/stream.m3u8
```

#### Browser (using hls.js)
```html
<video id="video" controls></video>
<script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
<script>
  const video = document.getElementById('video');
  const hls = new Hls();
  hls.loadSource('http://localhost:8080/stream/stream.m3u8');
  hls.attachMedia(video);
</script>
```

#### FFplay
```bash
ffplay http://localhost:8080/stream/stream.m3u8
```

## 📊 Database Schema

### availible_files
Stores information about available video files:
- `file_id` (PRIMARY KEY) - MD5 hash of filepath
- `filepath` - Full path to the file
- `file_size` - File size in bytes
- `video_length` - Video duration (can be populated with ffprobe)
- `added_time` - Unix timestamp when added
- `ffprobe_data` - JSON data from ffprobe

### video_queue
Manages the streaming queue:
- `id` (PRIMARY KEY) - Auto-increment ID
- `file_id` - Reference to availible_files
- `filepath` - Full path to the file
- `added_at` - Unix timestamp when added to queue
- `played` - Boolean (0 or 1)
- `played_at` - Unix timestamp when played
- `queue_position` - Position in queue
- `is_ad` - Boolean (0 or 1)

### play_history
Tracks playback history:
- `id` (PRIMARY KEY) - Auto-increment ID
- `file_id` - Reference to availible_files
- `filename` - File basename
- `filepath` - Full path to the file
- `started_at` - Unix timestamp when playback started
- `finished_at` - Unix timestamp when playback finished
- `duration_seconds` - Playback duration in seconds
- `is_ad` - Boolean (0 or 1)
- `skip_requested` - Boolean (0 or 1)

## 🔍 Detailed Logging

The application provides **comprehensive logging** at every step:

### Logging Levels
- **INFO**: General operations and successful actions
- **WARN**: Warnings and recoverable errors
- **ERROR**: Errors that need attention
- **DEBUG**: Detailed debugging information
- **TRACE**: Very detailed FFmpeg output

### Log Examples

#### Application Startup
```
INFO[0000] ffmpeg is not installed
INFO[0001] Starting ...
INFO[0001] loaded db path                               path="./database.db"
INFO[0002] ========================================
INFO[0002] Starting TV Streaming Service...
INFO[0002] ========================================
INFO[0002] Initializing TV Streamer Player...          module=streamer
INFO[0002] Player configuration loaded                  module=streamer output_dir=./out video_files_path=./videos ...
INFO[0002] Starting TV Streamer Player...              module=streamer
INFO[0002] Output directory created/verified           module=streamer path=./out
INFO[0002] Starting FFmpeg process...                  module=streamer
INFO[0002] ✓ FFmpeg process started successfully       module=streamer pid=12345 output_file=./out/stream.m3u8
```

#### Video Streaming
```
INFO[0010] ▶ Starting to stream video                  module=streamer video_id=1 file_id=abc123 filepath=/videos/movie.mp4
INFO[0010] Play history record created                 module=streamer history_id=1
INFO[0015] Streaming progress                          module=streamer bytes_copied=1048576 total_bytes=10485760 progress_pct=10.00%
INFO[0060] ✓ Video streaming completed successfully    module=streamer filepath=/videos/movie.mp4 bytes_streamed=10485760 duration=50.2s
```

#### API Requests
```
INFO[0100] Received request to skip to next video      module=web handler=handleStreamNext client_ip=127.0.0.1
INFO[0100] ⏭ Skipping current video                    module=streamer file_id=abc123 filepath=/videos/movie.mp4
INFO[0100] ✓ Successfully skipped to next video        module=web handler=handleStreamNext
```

### FFmpeg Output Monitoring
All FFmpeg output is captured and logged:
- Frame progress (DEBUG level)
- Input/Output information (INFO level)
- Warnings (WARN level)
- Errors (ERROR level)

## 🔧 Configuration

### Environment Variables
You can override config.yaml settings with environment variables:
```bash
export APP_APP_WEB_PORT=9090
export APP_APP_VIDEO_FILES_PATH=/custom/path
export APP_DATABASE_DB_PATH=/custom/db
export APP_STREAMING_OUTPUT_DIR=/custom/output
```

### Streaming Settings
- `output_dir`: Directory for HLS output files
- `hls_segment_time`: Duration of each HLS segment (seconds)
- `hls_list_size`: Number of segments in playlist
- `ffmpeg_preset`: FFmpeg encoding preset (ultrafast, veryfast, fast, medium, slow)
- `video_bitrate`: Video encoding bitrate (e.g., "2000k")
- `audio_bitrate`: Audio encoding bitrate (e.g., "128k")

## 📁 Project Structure

```
tv_streamer/
├── main.go                          # Application entry point
├── config.yaml                      # Configuration file
├── go.mod                           # Go module dependencies
├── helpers/
│   ├── config.go                    # Configuration loader
│   ├── db.go                        # Database connection
│   ├── checkw.go                    # Helper functions
│   └── logs/
│       └── instance.go              # Logger instance
├── migrations/
│   ├── migrations.go                # Migration runner
│   └── sql_files/
│       ├── 000001_initial_schema.up.sql
│       └── 000002_add_video_queue.up.sql
├── modules/
│   ├── streamer/
│   │   ├── run.go                   # Streamer initialization
│   │   ├── player.go                # FFmpeg player management
│   │   ├── queue.go                 # Queue management
│   │   └── models/
│   │       ├── video_queue.go
│   │       ├── play_history.go
│   │       └── available_files.go
│   └── web/
│       ├── run.go                   # Web server setup
│       └── stream_handlers.go       # API handlers
├── out/                             # HLS output directory (auto-created)
│   ├── stream.m3u8                  # HLS playlist
│   └── segment_*.ts                 # HLS segments
├── videos/                          # Video files directory
└── database.db                      # SQLite database
```

## 🐛 Troubleshooting

### FFmpeg Not Found
```
Error: ffmpeg is not installed
Solution: Install FFmpeg with libx264 and aac support
  Ubuntu/Debian: sudo apt-get install ffmpeg
  macOS: brew install ffmpeg
  Windows: Download from https://ffmpeg.org/download.html
```

### No Videos Playing
```
Check logs for: "No videos in queue, waiting 5 seconds..."
Solution: Add videos to the queue:
  1. PUT video files in the configured video_files_path
  2. Call POST /api/stream/scan?directory=/path/to/videos
  3. Or manually add: POST /api/stream/add?file=/path/to/video.mp4
```

### Stream Not Accessible
```
Check logs for: "✓ Web Server Started Successfully"
Solution:
  1. Verify port 8080 is not in use
  2. Check firewall settings
  3. Try accessing: http://localhost:8080/api/health
```

### FFmpeg Errors
```
Check logs for: "[FFMPEG ERROR]" messages
Solution: Check FFmpeg output in logs, common issues:
  - Unsupported codec: Re-encode video
  - Corrupted file: Verify file integrity
  - Permission denied: Check file permissions
```

## 📝 Example Workflow

1. **Start the application**
   ```bash
   go run main.go
   ```

2. **Add videos to queue**
   ```bash
   # Scan directory
   curl -X POST "http://localhost:8080/api/stream/scan?directory=./videos"

   # Or add individual files
   curl -X POST "http://localhost:8080/api/stream/add?file=./videos/movie1.mp4"
   curl -X POST "http://localhost:8080/api/stream/add?file=./videos/movie2.mp4"
   ```

3. **Start streaming**
   ```bash
   vlc http://localhost:8080/stream/stream.m3u8
   ```

4. **Control playback**
   ```bash
   # Skip current video
   curl -X POST "http://localhost:8080/api/stream/next"

   # Inject ad
   curl -X POST "http://localhost:8080/api/stream/inject-ad?file=./videos/ad.mp4"

   # Check queue
   curl "http://localhost:8080/api/stream/queue"

   # View history
   curl "http://localhost:8080/api/stream/history?limit=10"
   ```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License

This project is licensed under the MIT License.

## 🙏 Acknowledgments

- FFmpeg team for the powerful multimedia framework
- Go community for excellent libraries
- HLS protocol for streaming compatibility
