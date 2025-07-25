package main

import (
	"log"
	"os"

	"github.com/abhi14nexu/mercor-scd/internal/models"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "demo",
	Short: "SCD Library Demo CLI",
	Long: `Demo application showcasing the SCD (Slowly Changing Dimensions) library.

This CLI demonstrates how to work with versioned data using the SCD library,
including creating entities, updating them to create new versions, and querying
both latest and historical data.`,
	PersistentPreRun: initDB,
}

// initDB initializes the database connection
func initDB(cmd *cobra.Command, args []string) {
	var err error

	// Get DATABASE_URL from environment
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		// Default to SQLite for testing/demo purposes
		log.Println("DATABASE_URL not set, using SQLite for demo")
		db, err = gorm.Open(sqlite.Open("demo.db"), &gorm.Config{})
		if err != nil {
			log.Fatalf("Failed to connect to SQLite database: %v", err)
		}

		// Auto-migrate for SQLite
		if err := models.Register(db); err != nil {
			log.Fatalf("Failed to register models: %v", err)
		}

		log.Println("✅ Connected to SQLite database")
	} else {
		// Connect to PostgreSQL using DATABASE_URL
		db, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
		if err != nil {
			log.Fatalf("Failed to connect to PostgreSQL database: %v", err)
		}

		// Register models (validate schema)
		if err := models.Register(db); err != nil {
			log.Fatalf("Failed to register models: %v", err)
		}

		log.Println("✅ Connected to PostgreSQL database")
	}
}

func init() {
	// Add subcommands to root
	rootCmd.AddCommand(seedCmd)
	rootCmd.AddCommand(latestJobsCmd)
	rootCmd.AddCommand(paymentsCmd)
}
