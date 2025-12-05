package main

import (
	"batch-embedding-api/config"
	"batch-embedding-api/handlers"
	"batch-embedding-api/middleware"
	"batch-embedding-api/services"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set Gin mode based on environment
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize services
	embeddingService := services.NewEmbeddingService(cfg)
	jobStore := services.NewJobStore()
	worker := services.NewWorker(cfg, jobStore, embeddingService)

	// Start background workers
	worker.Start(5) // 5 concurrent workers

	// Initialize handlers
	handler := handlers.NewHandler(cfg, embeddingService, jobStore, worker)

	// Initialize rate limiter
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitPerSecond, cfg.RateLimitBurst)

	// Setup router
	router := gin.Default()

	// Apply global middleware
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.ErrorHandler())

	// Health endpoint (no auth required)
	router.GET("/v1/health", handler.Health)

	// API routes (with auth and rate limiting)
	api := router.Group("/v1")
	api.Use(middleware.AuthMiddleware(cfg))
	api.Use(middleware.RateLimitMiddleware(rateLimiter))
	{
		// Synchronous embedding
		api.POST("/embed", handler.Embed)

		// File upload embedding
		api.POST("/embed/file", handler.EmbedFile)

		// Async jobs
		api.POST("/jobs", handler.CreateJob)
		api.GET("/jobs", handler.ListJobs)
		api.GET("/jobs/:job_id", handler.GetJob)

		// Results
		api.GET("/results/:filename", handler.GetResults)
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("Shutting down...")
		worker.Stop()
		os.Exit(0)
	}()

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("🚀 Server starting on %s", addr)
	log.Printf("📚 API Documentation: http://localhost:%s/v1/health", cfg.Port)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
