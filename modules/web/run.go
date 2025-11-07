package web

import (
	"fmt"
	"time"
	"tv_streamer/helpers"
	"tv_streamer/helpers/logs"
	"tv_streamer/modules/streamer"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func Run() {
	logger := logs.GetLogger().WithField("module", "web")
	logger.Info("========================================")
	logger.Info("Starting Web Server...")
	logger.Info("========================================")

	// Initialize WebSocket Hub
	wsHub := GetWebSocketHub()

	// Add WebSocket hook to logger for broadcasting logs
	logs.GetLogger().AddHook(logs.NewWebSocketHook(wsHub))

	// Set broadcaster for streamer module to send currently_playing events
	streamer.SetBroadcaster(wsHub)

	router := gin.Default()

	// Configure and use CORS middleware
	config := cors.Config{
		AllowOrigins:     []string{"*"}, // Allow all origins, adjust as needed
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	router.Use(cors.New(config))

	// API routes
	api := router.Group("/api")
	{
		// Health check
		api.GET("/health", func(c *gin.Context) {
			logger.Debug("Health check requested")
			c.JSON(200, gin.H{
				"status":  true,
				"service": "tv_streamer",
				"version": "1.0.0",
			})
		})

		// WebSocket endpoint for debug messages
		api.GET("/ws", handleWebSocket)

		// Stream control endpoints
		stream := api.Group("/stream")
		{
			stream.POST("/next", handleStreamNext)
			stream.POST("/add", handleStreamAdd)
			stream.GET("/queue", handleStreamQueue)
			stream.GET("/status", handleStreamStatus)
			stream.POST("/inject-ad", handleInjectAd)
			stream.GET("/history", handleStreamHistory)
			stream.POST("/scan", handleScanVideos)
			stream.POST("/clear-played", handleClearPlayed)
		}

		// Schedule management endpoints
		schedule := api.Group("/schedule")
		{
			schedule.POST("/add", handleScheduleAdd)
			schedule.GET("/", handleScheduleGet)
			schedule.DELETE("/remove", handleScheduleRemove)
			schedule.POST("/clear", handleScheduleClear)
			schedule.POST("/reset", handleScheduleReset)
		}
	}

	// Serve HLS files
	router.Static("/stream", "./out")

	// Log available endpoints
	logger.Info("API Endpoints:")
	logger.Info("  GET  /api/health               - Health check")
	logger.Info("  GET  /api/ws                   - WebSocket debug API")
	logger.Info("")
	logger.Info("Stream Control:")
	logger.Info("  POST /api/stream/next          - Skip to next video")
	logger.Info("  POST /api/stream/add?file=...  - Add video to queue")
	logger.Info("  GET  /api/stream/queue         - Get current queue")
	logger.Info("  GET  /api/stream/status        - Get player status")
	logger.Info("  POST /api/stream/inject-ad?file=... - Inject ad")
	logger.Info("  GET  /api/stream/history?limit=50 - Get play history")
	logger.Info("  POST /api/stream/scan?directory=... - Scan directory")
	logger.Info("  POST /api/stream/clear-played  - Clear played items")
	logger.Info("")
	logger.Info("Schedule Management (Endless Loop):")
	logger.Info("  POST   /api/schedule/add?file=... - Add video to schedule")
	logger.Info("  GET    /api/schedule/          - Get current schedule")
	logger.Info("  DELETE /api/schedule/remove?file_id=... - Remove from schedule")
	logger.Info("  POST   /api/schedule/clear     - Clear schedule")
	logger.Info("  POST   /api/schedule/reset     - Reset schedule position")
	logger.Info("")
	logger.Info("HLS Stream:")
	logger.Info("  GET  /stream/stream.m3u8       - HLS playlist")
	logger.Info("")

	cfg := helpers.GetConfig()
	port := fmt.Sprintf(":%d", cfg.App.WebPort)

	logger.WithFields(logrus.Fields{
		"port": port,
		"mode": gin.Mode(),
	}).Info("========================================")
	logger.WithField("port", port).Info("✓ Web Server Started Successfully")
	logger.Info("========================================")
	logger.WithField("url", fmt.Sprintf("http://localhost%s/stream/stream.m3u8", port)).Info("Stream URL available at:")
	logger.WithField("url", fmt.Sprintf("http://localhost%s/api/health", port)).Info("API available at:")

	router.Run(port)
}
