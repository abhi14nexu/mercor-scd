package scd

import (
	"time"

	"gorm.io/gorm"
)

// Latest returns only the current/active versions (valid_to IS NULL)
// This is the most common query pattern (90% of use cases)
func Latest(db *gorm.DB) *gorm.DB {
	return db.Where("valid_to IS NULL")
}

// AsOf returns versions that were valid at the specified time
// Useful for point-in-time reporting and historical analysis
func AsOf(t time.Time) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("valid_from <= ? AND (valid_to IS NULL OR valid_to > ?)", t, t)
	}
}

// Historical returns all versions for analysis and audit trails
// This excludes the latest version and shows only historical records
func Historical(db *gorm.DB) *gorm.DB {
	return db.Where("valid_to IS NOT NULL")
}

// AllVersions returns all versions (both current and historical)
// Useful for complete audit trails and version analysis
func AllVersions(db *gorm.DB) *gorm.DB {
	return db // No filtering - returns everything
}

// ByBusinessID filters by the business identifier across all versions
// Useful when you need all versions of a specific business entity
func ByBusinessID(businessID string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("id = ?", businessID)
	}
}

// ByVersion filters by specific version number
// Useful for retrieving exact version of an entity
func ByVersion(version int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("version = ?", version)
	}
}

// ValidDuring returns versions that were valid during the specified time range
// Useful for period-based reporting and analysis
func ValidDuring(start, end time.Time) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(
			"valid_from <= ? AND (valid_to IS NULL OR valid_to >= ?)",
			end, start,
		)
	}
}

// CreatedAfter returns versions created after the specified time
// Useful for incremental processing and change tracking
func CreatedAfter(t time.Time) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("valid_from > ?", t)
	}
}

// CreatedBefore returns versions created before the specified time
// Useful for historical analysis and cleanup operations
func CreatedBefore(t time.Time) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("valid_from < ?", t)
	}
}

// OrderByVersion orders results by version number
// Can be combined with other scopes for consistent ordering
func OrderByVersion(desc bool) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if desc {
			return db.Order("version DESC")
		}
		return db.Order("version ASC")
	}
}

// OrderByTime orders results by temporal validity
// Useful for chronological analysis of changes
func OrderByTime(desc bool) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if desc {
			return db.Order("valid_from DESC")
		}
		return db.Order("valid_from ASC")
	}
}
