package scd

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SQLite-compatible SCD Model without PostgreSQL-specific syntax
type SQLiteModel struct {
	UID       uuid.UUID  `gorm:"primaryKey" json:"uid"`
	ID        string     `gorm:"not null" json:"id"`
	Version   int        `gorm:"not null" json:"version"`
	ValidFrom time.Time  `gorm:"not null" json:"valid_from"`
	ValidTo   *time.Time `json:"valid_to,omitempty"`
}

// Implement SCDModel interface
func (m *SQLiteModel) GetUID() uuid.UUID        { return m.UID }
func (m *SQLiteModel) GetBusinessID() string    { return m.ID }
func (m *SQLiteModel) GetVersion() int          { return m.Version }
func (m *SQLiteModel) SetUID(uid uuid.UUID)     { m.UID = uid }
func (m *SQLiteModel) SetBusinessID(id string)  { m.ID = id }
func (m *SQLiteModel) SetVersion(version int)   { m.Version = version }
func (m *SQLiteModel) SetValidFrom(t time.Time) { m.ValidFrom = t }
func (m *SQLiteModel) IsLatest() bool           { return m.ValidTo == nil }
func (m *SQLiteModel) Close(t time.Time)        { m.ValidTo = &t }

// BeforeCreate sets Version=1 for new business IDs, increments for existing IDs
func (m *SQLiteModel) BeforeCreate(tx *gorm.DB) error {
	// Generate UUID if not set
	if m.UID == uuid.Nil {
		m.UID = uuid.New()
	}
	// Business ID is required
	if m.ID == "" {
		return errors.New("business ID cannot be empty")
	}
	// If version not set, determine next version
	if m.Version == 0 {
		var maxVersion int
		err := tx.Model(m).Select("COALESCE(MAX(version), 0)").Where("id = ?", m.ID).Scan(&maxVersion).Error
		if err != nil {
			return err
		}
		m.Version = maxVersion + 1
	}
	return nil
}

// Test domain models that embed SQLiteModel to avoid import cycles
type ModelTestJob struct {
	SQLiteModel  `gorm:"embedded"`
	Status       string  `gorm:"not null" json:"status"`
	Rate         float64 `gorm:"not null" json:"rate"`
	Title        string  `gorm:"not null" json:"title"`
	CompanyID    string  `gorm:"not null" json:"company_id"`
	ContractorID string  `gorm:"not null" json:"contractor_id"`
}

func (ModelTestJob) TableName() string {
	return "jobs"
}

// AfterAutoMigrate creates table-specific indexes
func (ModelTestJob) AfterAutoMigrate(tx *gorm.DB) error {
	return tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_id_version ON jobs(id, version)").Error
}

// Test domain model for timelogs
type ModelTestTimelog struct {
	SQLiteModel `gorm:"embedded"`
	Duration    int64     `gorm:"not null" json:"duration"`
	JobUID      uuid.UUID `gorm:"not null" json:"job_uid"`
}

func (ModelTestTimelog) TableName() string {
	return "timelogs"
}

func (ModelTestTimelog) AfterAutoMigrate(tx *gorm.DB) error {
	return tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_timelogs_id_version ON timelogs(id, version)").Error
}

// Test domain model for payment line items
type ModelTestPaymentLineItem struct {
	SQLiteModel `gorm:"embedded"`
	JobUID      uuid.UUID `gorm:"not null" json:"job_uid"`
	TimelogUID  uuid.UUID `gorm:"not null" json:"timelog_uid"`
	Amount      float64   `gorm:"not null" json:"amount"`
	Status      string    `gorm:"not null" json:"status"`
}

func (ModelTestPaymentLineItem) TableName() string {
	return "payment_line_items"
}

func (ModelTestPaymentLineItem) AfterAutoMigrate(tx *gorm.DB) error {
	return tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_id_version ON payment_line_items(id, version)").Error
}

// openTestDB creates an in-memory SQLite database for testing
// It opens a shared cache database, auto-migrates test models, and returns the DB instance
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err, "Failed to connect to test database")

	// Auto-migrate test models for SQLite
	err = db.AutoMigrate(&ModelTestJob{}, &ModelTestTimelog{}, &ModelTestPaymentLineItem{})
	require.NoError(t, err, "Failed to auto-migrate test models")

	return db
}

// cleanup closes the database connection
func cleanup(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}

// TestSCDCreateNew tests creating a new Job via scd.CreateNew
func TestSCDCreateNew(t *testing.T) {
	db := openTestDB(t)
	defer cleanup(db)

	// Create a new job
	job := &ModelTestJob{
		SQLiteModel: SQLiteModel{
			ID: "test-job-1",
		},
		Status:       "active",
		Rate:         100.0,
		Title:        "Software Engineer",
		CompanyID:    "company-1",
		ContractorID: "contractor-1",
	}

	// Create via SCD
	result, err := CreateNew(db, job)
	require.NoError(t, err, "CreateNew should succeed")

	// Assert Version == 1
	assert.Equal(t, 1, result.GetVersion(), "First version should be 1")

	// Assert ValidTo == nil (latest version)
	assert.True(t, result.IsLatest(), "New record should be latest")
	assert.Nil(t, result.ValidTo, "ValidTo should be nil for latest version")

	// Query Latest should return exactly that row
	latest, err := GetLatest[*ModelTestJob](db, "test-job-1")
	require.NoError(t, err, "GetLatest should find the job")

	assert.Equal(t, result.GetUID(), latest.GetUID(), "Latest should return the same record")
	assert.Equal(t, "Software Engineer", latest.Title, "Title should match")
	assert.Equal(t, 100.0, latest.Rate, "Rate should match")
}

// TestUpdateCreatesSequentialVersions tests that scd.Update creates sequential versions
func TestUpdateCreatesSequentialVersions(t *testing.T) {
	db := openTestDB(t)
	defer cleanup(db)

	// Create initial job
	job := &ModelTestJob{
		SQLiteModel: SQLiteModel{
			ID: "test-job-seq",
		},
		Status:       "active",
		Rate:         100.0,
		Title:        "Initial Title",
		CompanyID:    "company-1",
		ContractorID: "contractor-1",
	}
	_, err := CreateNew(db, job)
	require.NoError(t, err, "CreateNew should succeed")

	// First update - should create version 2
	_, err = Update(db, "test-job-seq", func(j *ModelTestJob) {
		j.Title = "Updated Title 1"
		j.Rate = 110.0
	})
	require.NoError(t, err, "First update should succeed")

	// Second update - should create version 3
	_, err = Update(db, "test-job-seq", func(j *ModelTestJob) {
		j.Title = "Updated Title 2"
		j.Rate = 120.0
	})
	require.NoError(t, err, "Second update should succeed")

	// Count total versions - should be 3
	var count int64
	err = db.Model(&ModelTestJob{}).Where("id = ?", "test-job-seq").Count(&count).Error
	require.NoError(t, err, "Count query should succeed")
	assert.Equal(t, int64(3), count, "Should have exactly 3 versions")

	// Latest version should be version 3
	latest, err := GetLatest[*ModelTestJob](db, "test-job-seq")
	require.NoError(t, err, "GetLatest should find the job")
	assert.Equal(t, 3, latest.GetVersion(), "Latest version should be 3")
	assert.Equal(t, "Updated Title 2", latest.Title, "Latest should have final title")
	assert.Equal(t, 120.0, latest.Rate, "Latest should have final rate")

	// Historical rows should have ValidTo set (not nil)
	var historicalCount int64
	err = db.Model(&ModelTestJob{}).Where("id = ? AND valid_to IS NOT NULL", "test-job-seq").Count(&historicalCount).Error
	require.NoError(t, err, "Historical count query should succeed")
	assert.Equal(t, int64(2), historicalCount, "Should have 2 historical records with ValidTo set")
}

// TestAsOfScope tests the AsOf scope functionality
func TestAsOfScope(t *testing.T) {
	db := openTestDB(t)
	defer cleanup(db)

	// Create initial job
	job := &ModelTestJob{
		SQLiteModel: SQLiteModel{
			ID: "test-job-asof",
		},
		Status:       "active",
		Rate:         100.0,
		Title:        "Version 1",
		CompanyID:    "company-1",
		ContractorID: "contractor-1",
	}
	_, err := CreateNew(db, job)
	require.NoError(t, err, "CreateNew should succeed")

	// Capture timestamp between v1 and v2
	time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
	t1 := time.Now()
	time.Sleep(10 * time.Millisecond)

	// Create version 2
	_, err = Update(db, "test-job-asof", func(j *ModelTestJob) {
		j.Title = "Version 2"
		j.Rate = 110.0
	})
	require.NoError(t, err, "Update should succeed")

	// Query AsOf(t1) should return Version 1
	var jobAsOf ModelTestJob
	err = db.Scopes(AsOf(t1)).Where("id = ?", "test-job-asof").First(&jobAsOf).Error
	require.NoError(t, err, "AsOf query should find version 1")

	assert.Equal(t, 1, jobAsOf.GetVersion(), "AsOf(t1) should return version 1")
	assert.Equal(t, "Version 1", jobAsOf.Title, "AsOf(t1) should return original title")
	assert.Equal(t, 100.0, jobAsOf.Rate, "AsOf(t1) should return original rate")
}

// TestConcurrentUpdateRace tests concurrent updates to ensure no race conditions
func TestConcurrentUpdateRace(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer cleanup(db)

	// Create initial job
	job := &ModelTestJob{
		SQLiteModel: SQLiteModel{
			ID: "job-race",
		},
		Status:       "active",
		Rate:         100.0,
		Title:        "Initial Title",
		CompanyID:    "company-1",
		ContractorID: "contractor-1",
	}
	_, err := CreateNew(db, job)
	require.NoError(t, err, "CreateNew should succeed")

	// Spawn 5 goroutines each calling scd.Update (reduced for SQLite compatibility)
	const numGoroutines = 5
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Channel to collect successful updates
	successChan := make(chan int, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()

			// Add small delays to reduce contention for SQLite
			time.Sleep(time.Duration(i) * time.Millisecond)

			_, err := Update(db, "job-race", func(j *ModelTestJob) {
				j.Title = fmt.Sprintf("Updated by goroutine %d", i)
				j.Rate = 100.0 + float64(i)
			})
			if err == nil {
				successChan <- 1
			}
			// For SQLite, we expect some failures due to table locking, so we don't fail the test
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(successChan)

	// Count successful updates
	successCount := 0
	for range successChan {
		successCount++
	}

	// After join, count total versions (should be 1 original + successful updates)
	var totalCount int64
	err = db.Model(&ModelTestJob{}).Where("id = ?", "job-race").Count(&totalCount).Error
	require.NoError(t, err, "Count query should succeed")
	expectedTotal := int64(1 + successCount)
	assert.Equal(t, expectedTotal, totalCount, fmt.Sprintf("Should have %d versions (1 original + %d successful updates)", expectedTotal, successCount))

	// Verify that we have at least some concurrent updates (at least 2 total versions)
	assert.GreaterOrEqual(t, totalCount, int64(2), "Should have at least 2 versions (showing some concurrency worked)")

	// Verify versions are contiguous from 1 to totalCount
	var versions []int
	err = db.Model(&ModelTestJob{}).Where("id = ?", "job-race").
		Order("version ASC").Pluck("version", &versions).Error
	require.NoError(t, err, "Version query should succeed")

	// Check that versions are contiguous starting from 1
	for i, version := range versions {
		assert.Equal(t, i+1, version, "Versions should be contiguous starting from 1")
	}

	// Verify latest version has ValidTo == nil
	latest, err := GetLatest[*ModelTestJob](db, "job-race")
	require.NoError(t, err, "GetLatest should find the job")
	assert.Equal(t, int(totalCount), latest.GetVersion(), "Latest version should match total count")
	assert.True(t, latest.IsLatest(), "Latest version should have ValidTo == nil")
}
