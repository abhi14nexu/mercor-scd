package main

import (
	"log"
	"os"

	"github.com/abhi14nexu/mercor-scd/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// CORS middleware function for the dashboard
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func main() {
	// Get DATABASE_URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	// Connect to PostgreSQL
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL database: %v", err)
	}

	// Register models (validate schema)
	if err := models.Register(db); err != nil {
		log.Fatalf("Failed to register models: %v", err)
	}

	log.Println("✅ Connected to PostgreSQL database")

	// Create Gin router
	router := gin.Default()

	// Add CORS middleware
	router.Use(corsMiddleware())

	// Setup API routes
	api := router.Group("/api/v1")
	{
		// Jobs endpoints
		api.GET("/jobs", getJobs(db))
		api.GET("/jobs/:id", getJob(db))
		api.GET("/jobs/:id/versions", getJobVersions(db))

		// Payment line items endpoints
		api.GET("/payments", getPayments(db))
		api.GET("/payments/:id", getPayment(db))
		api.GET("/payments/:id/versions", getPaymentVersions(db))

		// Timelogs endpoints
		api.GET("/timelogs", getTimelogs(db))
		api.GET("/timelogs/:id", getTimelog(db))
		api.GET("/timelogs/:id/versions", getTimelogVersions(db))

		// Health check
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "healthy"})
		})
	}

	// Start server on port 8081 (8080 seems to be in use)
	log.Println("🚀 Starting API server on :8081")
	if err := router.Run(":8081"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
