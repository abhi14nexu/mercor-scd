package scd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openTestDB creates an in-memory SQLite database for testing
// Same as model_test.go but defined separately to avoid duplication issues
func openIntegrityTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err, "Failed to connect to test database")

	// Auto-migrate test models for SQLite
	err = db.AutoMigrate(&ModelTestJob{}, &ModelTestTimelog{}, &ModelTestPaymentLineItem{})
	require.NoError(t, err, "Failed to auto-migrate test models")

	return db
}

// seedSampleData creates sample data for integrity tests
// Creates one Job with 3 versions, one Timelog referencing v2, one PaymentLineItem
func seedSampleData(db *gorm.DB) error {
	// Create initial job (version 1)
	job1 := &ModelTestJob{
		SQLiteModel: SQLiteModel{
			ID: "integrity-job-1",
		},
		Status:       "active",
		Rate:         100.0,
		Title:        "Job Version 1",
		CompanyID:    "company-1",
		ContractorID: "contractor-1",
	}
	_, err := CreateNew(db, job1)
	if err != nil {
		return err
	}

	// Create version 2
	v2Job, err := Update(db, "integrity-job-1", func(j *ModelTestJob) {
		j.Title = "Job Version 2"
		j.Rate = 110.0
	})
	if err != nil {
		return err
	}

	// Create version 3
	_, err = Update(db, "integrity-job-1", func(j *ModelTestJob) {
		j.Title = "Job Version 3"
		j.Rate = 120.0
	})
	if err != nil {
		return err
	}

	// Create timelog referencing version 2
	timelog := &ModelTestTimelog{
		SQLiteModel: SQLiteModel{
			ID: "timelog-1",
		},
		Duration: 3600000, // 1 hour in milliseconds
		JobUID:   v2Job.GetUID(),
	}
	createdTimelog, err := CreateNew(db, timelog)
	if err != nil {
		return err
	}

	// Create payment line item referencing the timelog and job v2
	paymentItem := &ModelTestPaymentLineItem{
		SQLiteModel: SQLiteModel{
			ID: "payment-1",
		},
		JobUID:     v2Job.GetUID(),
		TimelogUID: createdTimelog.GetUID(),
		Amount:     110.0, // 1 hour * $110/hour
		Status:     "not-paid",
	}
	_, err = CreateNew(db, paymentItem)
	if err != nil {
		return err
	}

	return nil
}

// TestNoDuplicateVersion tests that there are no duplicate (id, version) pairs
func TestNoDuplicateVersion(t *testing.T) {
	db := openIntegrityTestDB(t)
	defer cleanup(db)

	// Seed test data
	err := seedSampleData(db)
	require.NoError(t, err, "Failed to seed sample data")

	// Run integrity check: no duplicate (id, version) pairs
	var count int64
	err = db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT 1 FROM jobs 
			GROUP BY id, version 
			HAVING COUNT(*) > 1 
			LIMIT 1
		) duplicates
	`).Scan(&count).Error
	require.NoError(t, err, "Duplicate version query should succeed")

	assert.Equal(t, int64(0), count, "Should have no duplicate (id, version) pairs")
}

// TestSingleLatestRow tests that each business ID has exactly one latest row (valid_to IS NULL)
func TestSingleLatestRow(t *testing.T) {
	db := openIntegrityTestDB(t)
	defer cleanup(db)

	// Seed test data
	err := seedSampleData(db)
	require.NoError(t, err, "Failed to seed sample data")

	// Run integrity check: each ID should have exactly one latest row
	var count int64
	err = db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT id FROM jobs 
			WHERE valid_to IS NULL
			GROUP BY id
			HAVING COUNT(*) != 1
		) invalid_latest
	`).Scan(&count).Error
	require.NoError(t, err, "Single latest row query should succeed")

	assert.Equal(t, int64(0), count, "Each business ID should have exactly one latest row")
}

// TestNoGaps tests that version numbers are contiguous (no gaps)
func TestNoGaps(t *testing.T) {
	db := openIntegrityTestDB(t)
	defer cleanup(db)

	// Seed test data
	err := seedSampleData(db)
	require.NoError(t, err, "Failed to seed sample data")

	// Run integrity check: no gaps in version numbers
	var count int64
	err = db.Raw(`
		WITH seq AS (
			SELECT 
				id, 
				MIN(version) as vmin, 
				MAX(version) as vmax, 
				COUNT(*) as cnt
			FROM jobs 
			GROUP BY id
		)
		SELECT COUNT(*) FROM seq 
		WHERE (vmax - vmin + 1) != cnt
	`).Scan(&count).Error
	require.NoError(t, err, "No gaps query should succeed")

	assert.Equal(t, int64(0), count, "Should have no gaps in version sequences")
}

// TestNoOverlap tests that validity periods don't overlap
func TestNoOverlap(t *testing.T) {
	db := openIntegrityTestDB(t)
	defer cleanup(db)

	// Seed test data
	err := seedSampleData(db)
	require.NoError(t, err, "Failed to seed sample data")

	// Run integrity check: no overlapping validity periods
	var count int64
	err = db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT 1 FROM jobs j1 
			JOIN jobs j2 ON j1.id = j2.id AND j1.uid != j2.uid
			WHERE j1.valid_to IS NOT NULL 
			  AND j2.valid_from < j1.valid_to
			  AND j2.valid_from >= j1.valid_from
			LIMIT 1
		) overlaps
	`).Scan(&count).Error
	require.NoError(t, err, "No overlap query should succeed")

	assert.Equal(t, int64(0), count, "Should have no overlapping validity periods")
}

// TestForeignKeyIntegrity tests FK consistency for timelogs and payment_line_items
func TestForeignKeyIntegrity(t *testing.T) {
	db := openIntegrityTestDB(t)
	defer cleanup(db)

	// Seed test data
	err := seedSampleData(db)
	require.NoError(t, err, "Failed to seed sample data")

	// Test 1: No dangling timelogs (all job_uid references should exist)
	var danglingTimelogs int64
	err = db.Raw(`
		SELECT COUNT(*) FROM timelogs t
		LEFT JOIN jobs j ON t.job_uid = j.uid
		WHERE j.uid IS NULL
	`).Scan(&danglingTimelogs).Error
	require.NoError(t, err, "Dangling timelogs query should succeed")
	assert.Equal(t, int64(0), danglingTimelogs, "Should have no dangling timelog references to jobs")

	// Test 2: No dangling payment_line_items -> jobs
	var danglingPaymentsToJobs int64
	err = db.Raw(`
		SELECT COUNT(*) FROM payment_line_items p
		LEFT JOIN jobs j ON p.job_uid = j.uid
		WHERE j.uid IS NULL
	`).Scan(&danglingPaymentsToJobs).Error
	require.NoError(t, err, "Dangling payments to jobs query should succeed")
	assert.Equal(t, int64(0), danglingPaymentsToJobs, "Should have no dangling payment references to jobs")

	// Test 3: No dangling payment_line_items -> timelogs
	var danglingPaymentsToTimelogs int64
	err = db.Raw(`
		SELECT COUNT(*) FROM payment_line_items p
		LEFT JOIN timelogs t ON p.timelog_uid = t.uid
		WHERE t.uid IS NULL
	`).Scan(&danglingPaymentsToTimelogs).Error
	require.NoError(t, err, "Dangling payments to timelogs query should succeed")
	assert.Equal(t, int64(0), danglingPaymentsToTimelogs, "Should have no dangling payment references to timelogs")

	// Test 4: Verify our test data is actually there
	var jobCount, timelogCount, paymentCount int64
	db.Model(&ModelTestJob{}).Count(&jobCount)
	db.Model(&ModelTestTimelog{}).Count(&timelogCount)
	db.Model(&ModelTestPaymentLineItem{}).Count(&paymentCount)

	assert.Equal(t, int64(3), jobCount, "Should have 3 job versions")
	assert.Equal(t, int64(1), timelogCount, "Should have 1 timelog")
	assert.Equal(t, int64(1), paymentCount, "Should have 1 payment line item")
}
