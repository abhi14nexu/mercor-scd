package models

import (
	"fmt"

	"gorm.io/gorm"
)

// Register sets up database models for the given GORM DB instance
// For SQLite (tests): runs AutoMigrate to create tables
// For PostgreSQL (production): relies on migration files
func Register(db *gorm.DB) error {
	if db.Dialector.Name() == "sqlite" {
		// Test environment - create tables automatically
		return autoMigrateModels(db)
	}

	// Production environment - tables managed by migration files
	// Just validate that models are properly configured
	return validateModels(db)
}

// autoMigrateModels creates all tables for testing environment
func autoMigrateModels(db *gorm.DB) error {
	// For SQLite testing, we need to use simpler field types
	// The SCD Model will automatically adapt its UUID and index handling
	models := []interface{}{
		&Job{},
		&Timelog{},
		&PaymentLineItem{},
	}

	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			// For SQLite compatibility, ignore specific syntax errors
			if db.Dialector.Name() == "sqlite" {
				// Log the error but continue - SQLite may not support all PostgreSQL features
				fmt.Printf("SQLite migration warning for %T: %v\n", model, err)
				continue
			}
			return fmt.Errorf("failed to migrate model %T: %w", model, err)
		}
	}

	return nil
}

// validateModels performs basic validation that models are properly configured
func validateModels(db *gorm.DB) error {
	// Test that models can be parsed by GORM
	models := []interface{}{
		&Job{},
		&Timelog{},
		&PaymentLineItem{},
	}

	for _, model := range models {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(model); err != nil {
			return fmt.Errorf("model validation failed for %T: %w", model, err)
		}
	}

	return nil
}

// GetAllModels returns a slice of all model types for registration
func GetAllModels() []interface{} {
	return []interface{}{
		&Job{},
		&Timelog{},
		&PaymentLineItem{},
	}
}

// TableNames returns a slice of all table names used by the models
func TableNames() []string {
	return []string{
		Job{}.TableName(),
		Timelog{}.TableName(),
		PaymentLineItem{}.TableName(),
	}
}
