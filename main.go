package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/victoryann-claw/code-review-bot/internal/config"
	"github.com/victoryann-claw/code-review-bot/internal/handler"
)

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	if err := config.LoadConfig(configPath); err != nil {
		log.Printf("Warning: Failed to load config from %s: %v, using defaults", configPath, err)
	}

	// Set Gin mode
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize router
	r := gin.Default()

	// Health check endpoint
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "CodeReviewBot is running",
		})
	})

	// GitHub Webhook endpoint
	r.POST("/webhook", handler.HandleWebhook)

	// Get port from config or environment
	cfg := config.GetConfig()
	port := "3000"
	if cfg != nil && cfg.Server.Port != "" {
		port = cfg.Server.Port
	}
	if portEnv := os.Getenv("PORT"); portEnv != "" {
		port = portEnv
	}

	log.Printf("CodeReviewBot is listening on port %s", port)
	log.Printf("Configure your GitHub webhook URL to: http://<your-server>/webhook")

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
