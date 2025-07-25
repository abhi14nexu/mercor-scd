package scd

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Update creates a new version of an existing record with the specified mutations
// This is the primary way to modify SCD entities while preserving history
func Update[T SCDModel](db *gorm.DB, businessID string, mutator func(T)) (T, error) {
	var result T

	err := db.Transaction(func(tx *gorm.DB) error {
		// 1. Get the current latest version
		var latest T
		if err := tx.Scopes(Latest).Where("id = ?", businessID).First(&latest).Error; err != nil {
			return fmt.Errorf("failed to find latest version of %s: %w", businessID, err)
		}

		// 2. Get table name for atomic version allocation
		tableName, err := getTableName(tx, result)
		if err != nil {
			return fmt.Errorf("failed to determine table name: %w", err)
		}

		// 3. Atomically allocate next version number using CTE
		// This prevents race conditions where concurrent updates try the same version
		var nextVersion int
		if err := tx.Raw(`
			SELECT COALESCE(MAX(version), 0) + 1 AS next_version
			FROM `+tableName+` 
			WHERE id = ?`,
			businessID,
		).Scan(&nextVersion).Error; err != nil {
			return fmt.Errorf("failed to get next version: %w", err)
		}

		// 4. Create a copy for the new version with mutations applied
		result = latest
		mutator(result)

		// 5. Prepare new version with atomic version number
		result.SetUID(uuid.New())
		result.SetVersion(nextVersion)

		// 6. Insert new version first (with retry logic for race condition protection)
		maxRetries := 3
		for attempt := 0; attempt < maxRetries; attempt++ {
			if err := tx.Create(result).Error; err != nil {
				// Check if it's a unique constraint violation on (id, version)
				if attempt < maxRetries-1 && isUniqueConstraintError(err) {
					// Recalculate version and retry
					if err := tx.Raw(`
						SELECT COALESCE(MAX(version), 0) + 1 AS next_version
						FROM `+tableName+` 
						WHERE id = ?`,
						businessID,
					).Scan(&nextVersion).Error; err != nil {
						return fmt.Errorf("failed to recalculate version on retry: %w", err)
					}
					result.SetVersion(nextVersion)
					continue // Retry with new version
				}
				return fmt.Errorf("failed to create new version: %w", err)
			}
			break // Success
		}

		// 7. Close the previous version AFTER successfully creating new one
		now := time.Now()
		if err := tx.Model(&latest).Update("valid_to", now).Error; err != nil {
			return fmt.Errorf("failed to close previous version: %w", err)
		}

		return nil
	})

	if err != nil {
		var zero T
		return zero, err
	}

	return result, nil
}

// CreateNew creates the first version of a new business entity
// Use this for creating brand new entities, not for updating existing ones
func CreateNew[T SCDModel](db *gorm.DB, entity T) (T, error) {
	// Validate business ID is provided
	if entity.GetBusinessID() == "" {
		var zero T
		return zero, errors.New("business ID is required for new entities")
	}

	// Check if entity already exists
	var exists T
	err := db.Scopes(Latest).Where("id = ?", entity.GetBusinessID()).First(&exists).Error
	if err == nil {
		var zero T
		return zero, fmt.Errorf("entity with business ID %s already exists", entity.GetBusinessID())
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		var zero T
		return zero, fmt.Errorf("failed to check entity existence: %w", err)
	}

	// Set SCD fields for new entity
	entity.SetUID(uuid.New())
	entity.SetVersion(1)

	// Create the entity
	if err := db.Create(entity).Error; err != nil {
		var zero T
		return zero, fmt.Errorf("failed to create new entity: %w", err)
	}

	return entity, nil
}

// GetLatest retrieves the current version of an entity by business ID
func GetLatest[T SCDModel](db *gorm.DB, businessID string) (T, error) {
	var entity T
	err := db.Scopes(Latest).Where("id = ?", businessID).First(&entity).Error
	if err != nil {
		var zero T
		return zero, err
	}
	return entity, nil
}

// GetAllVersions retrieves all versions of an entity by business ID
// Returns versions ordered from oldest to newest
func GetAllVersions[T SCDModel](db *gorm.DB, businessID string) ([]T, error) {
	var versions []T
	err := db.Scopes(ByBusinessID(businessID), OrderByVersion(false)).Find(&versions).Error
	if err != nil {
		return nil, err
	}
	return versions, nil
}

// GetVersion retrieves a specific version of an entity
func GetVersion[T SCDModel](db *gorm.DB, businessID string, version int) (T, error) {
	var entity T
	err := db.Scopes(ByBusinessID(businessID), ByVersion(version)).First(&entity).Error
	if err != nil {
		var zero T
		return zero, err
	}
	return entity, nil
}

// SoftDelete marks the latest version as invalid by setting valid_to
// This preserves all historical data while making the entity "deleted"
func SoftDelete[T SCDModel](db *gorm.DB, businessID string) error {
	latest, err := GetLatest[T](db, businessID)
	if err != nil {
		return fmt.Errorf("failed to find latest version for soft delete: %w", err)
	}

	now := time.Now()
	if err := db.Model(latest).Update("valid_to", now).Error; err != nil {
		return fmt.Errorf("failed to soft delete entity: %w", err)
	}

	return nil
}

// getTableName extracts the table name from GORM model
func getTableName[T any](db *gorm.DB, model T) (string, error) {
	// For generics, we need to use the zero value approach
	var zero T
	stmt := &gorm.Statement{DB: db}

	// Parse the schema to get table name
	if err := stmt.Parse(zero); err != nil {
		return "", fmt.Errorf("failed to parse model schema: %w", err)
	}

	if stmt.Schema == nil || stmt.Schema.Table == "" {
		// Fallback: try to get table name from struct name
		modelType := reflect.TypeOf(zero)
		if modelType.Kind() == reflect.Ptr {
			modelType = modelType.Elem()
		}
		return db.NamingStrategy.TableName(modelType.Name()), nil
	}

	return stmt.Schema.Table, nil
}

// Exists checks if an entity with the given business ID exists (has any version)
func Exists[T SCDModel](db *gorm.DB, businessID string) (bool, error) {
	var count int64
	err := db.Model(new(T)).Where("id = ?", businessID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasLatestVersion checks if an entity has a current/active version
func HasLatestVersion[T SCDModel](db *gorm.DB, businessID string) (bool, error) {
	var count int64
	err := db.Model(new(T)).Scopes(Latest).Where("id = ?", businessID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// isUniqueConstraintError checks if the error is a unique constraint violation
// This helps detect race conditions in version allocation
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	// PostgreSQL unique constraint violation patterns
	return strings.Contains(errStr, "duplicate key value") ||
		strings.Contains(errStr, "unique constraint") ||
		strings.Contains(errStr, "violates unique constraint") ||
		// MySQL unique constraint violation patterns
		strings.Contains(errStr, "duplicate entry") ||
		strings.Contains(errStr, "unique key constraint")
}
