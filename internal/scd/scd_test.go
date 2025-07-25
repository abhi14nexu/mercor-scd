package scd

import (
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

// Test domain models that embed scd.Model
type TestJob struct {
	Model
	Status string  `json:"status"`
	Rate   float64 `json:"rate"`
	Title  string  `json:"title"`
}

type TestTimelog struct {
	Model
	Duration int64     `json:"duration"`
	JobUID   uuid.UUID `json:"job_uid"`
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "Failed to connect to test database")

	// Auto-migrate our test models
	err = db.AutoMigrate(&TestJob{}, &TestTimelog{})
	require.NoError(t, err, "Failed to migrate test models")

	return db
}

// Test CreateNew function
func TestCreateNew(t *testing.T) {
	db := setupTestDB(t)

	job := &TestJob{
		Model: Model{
			ID: "test-job-1",
		},
		Status: "active",
		Rate:   50.0,
		Title:  "Software Engineer",
	}

	createdJob, err := CreateNew[*TestJob](db, job)
	require.NoError(t, err, "CreateNew should succeed")

	// Assertions
	assert.Equal(t, 1, createdJob.GetVersion(), "First version should be 1")
	assert.Nil(t, createdJob.ValidTo, "ValidTo should be nil for latest version")
	assert.Equal(t, "test-job-1", createdJob.GetBusinessID(), "Business ID should match")
	assert.NotEqual(t, uuid.Nil, createdJob.GetUID(), "UID should be generated")
	assert.Equal(t, "active", createdJob.Status, "Status should be preserved")
	assert.Equal(t, 50.0, createdJob.Rate, "Rate should be preserved")

	// Test duplicate creation fails
	_, err = CreateNew[*TestJob](db, &TestJob{Model: Model{ID: "test-job-1"}})
	assert.Error(t, err, "Creating duplicate entity should fail")
	assert.Contains(t, err.Error(), "already exists", "Error should mention entity already exists")
}

// Test Update flow with sequential versions
func TestUpdateFlow(t *testing.T) {
	db := setupTestDB(t)

	// Create initial job
	job := &TestJob{
		Model:  Model{ID: "test-job-2"},
		Status: "active",
		Rate:   50.0,
		Title:  "Engineer",
	}

	createdJob, err := CreateNew[*TestJob](db, job)
	require.NoError(t, err)
	assert.Equal(t, 1, createdJob.GetVersion())

	// First update
	updatedJob1, err := Update[*TestJob](db, "test-job-2", func(j *TestJob) {
		j.Rate = 60.0
		j.Status = "promoted"
	})
	require.NoError(t, err, "First update should succeed")
	assert.Equal(t, 2, updatedJob1.GetVersion(), "Second version should be 2")
	assert.Equal(t, 60.0, updatedJob1.Rate, "Rate should be updated")
	assert.Equal(t, "promoted", updatedJob1.Status, "Status should be updated")

	// Second update
	updatedJob2, err := Update[*TestJob](db, "test-job-2", func(j *TestJob) {
		j.Rate = 70.0
		j.Title = "Senior Engineer"
	})
	require.NoError(t, err, "Second update should succeed")
	assert.Equal(t, 3, updatedJob2.GetVersion(), "Third version should be 3")
	assert.Equal(t, 70.0, updatedJob2.Rate, "Rate should be updated again")
	assert.Equal(t, "Senior Engineer", updatedJob2.Title, "Title should be updated")

	// Verify all versions exist
	var allVersions []TestJob
	err = db.Scopes(ByBusinessID("test-job-2"), OrderByVersion(false)).Find(&allVersions).Error
	require.NoError(t, err)
	assert.Len(t, allVersions, 3, "Should have 3 versions")

	// Verify version sequence
	assert.Equal(t, 1, allVersions[0].GetVersion())
	assert.Equal(t, 2, allVersions[1].GetVersion())
	assert.Equal(t, 3, allVersions[2].GetVersion())

	// Verify ValidTo handling
	assert.NotNil(t, allVersions[0].ValidTo, "Version 1 should have ValidTo set")
	assert.NotNil(t, allVersions[1].ValidTo, "Version 2 should have ValidTo set")
	assert.Nil(t, allVersions[2].ValidTo, "Version 3 (latest) should have ValidTo nil")

	// Verify Latest scope returns only version 3
	var latestJob TestJob
	err = db.Scopes(Latest).Where("id = ?", "test-job-2").First(&latestJob).Error
	require.NoError(t, err)
	assert.Equal(t, 3, latestJob.GetVersion(), "Latest should return version 3")
	assert.Equal(t, 70.0, latestJob.Rate, "Latest should have newest rate")
}

// Critical concurrency test - this is the most important test!
func TestConcurrentUpdates(t *testing.T) {
	db := setupTestDB(t)

	// Create initial job
	job := &TestJob{
		Model:  Model{ID: "concurrent-job"},
		Status: "active",
		Rate:   50.0,
	}

	_, err := CreateNew[*TestJob](db, job)
	require.NoError(t, err)

	// Launch 10 concurrent updates
	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make([]*TestJob, numGoroutines)
	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			updatedJob, err := Update[*TestJob](db, "concurrent-job", func(j *TestJob) {
				j.Rate = float64(100 + index) // Each goroutine sets different rate
				j.Status = fmt.Sprintf("updated-%d", index)
			})

			results[index] = updatedJob
			errors[index] = err
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify all updates succeeded
	for i, err := range errors {
		assert.NoError(t, err, "Update %d should succeed", i)
	}

	// Retrieve all versions and verify
	var allVersions []TestJob
	err = db.Scopes(ByBusinessID("concurrent-job"), OrderByVersion(false)).Find(&allVersions).Error
	require.NoError(t, err)

	// Should have initial version + 10 updates = 11 total versions
	assert.Len(t, allVersions, 11, "Should have 11 versions total (1 initial + 10 updates)")

	// Verify sequential version numbers with no gaps or duplicates
	for i, version := range allVersions {
		expectedVersion := i + 1
		assert.Equal(t, expectedVersion, version.GetVersion(),
			"Version %d should have version number %d", i, expectedVersion)
	}

	// Verify only the last version has ValidTo == nil
	for i, version := range allVersions {
		if i == len(allVersions)-1 {
			assert.Nil(t, version.ValidTo, "Last version should have ValidTo nil")
		} else {
			assert.NotNil(t, version.ValidTo, "Historical version %d should have ValidTo set", i+1)
		}
	}

	// Verify each update result has the correct version number
	resultVersions := make(map[int]bool)
	for _, result := range results {
		if result != nil {
			resultVersions[result.GetVersion()] = true
		}
	}

	// Should have versions 2 through 11 (version 1 was the initial creation)
	for v := 2; v <= 11; v++ {
		assert.True(t, resultVersions[v], "Should have result with version %d", v)
	}
}

// Test BeforeUpdate guards prevent direct SCD field modification
func TestBeforeUpdateGuard(t *testing.T) {
	db := setupTestDB(t)

	// Create test job
	job := &TestJob{
		Model:  Model{ID: "guard-test"},
		Status: "active",
	}

	createdJob, err := CreateNew[*TestJob](db, job)
	require.NoError(t, err)

	// Attempt to directly modify protected fields
	createdJob.Version = 99
	err = db.Save(createdJob).Error
	assert.Error(t, err, "Direct version modification should fail")
	assert.Contains(t, err.Error(), "Version cannot be modified", "Error should mention version protection")

	// Reset and try UID modification
	createdJob.Version = 1 // Reset to original
	originalUID := createdJob.UID
	createdJob.UID = uuid.New()
	err = db.Save(createdJob).Error
	assert.Error(t, err, "Direct UID modification should fail")
	assert.Contains(t, err.Error(), "UID cannot be modified", "Error should mention UID protection")

	// Reset and try ValidFrom modification
	createdJob.UID = originalUID // Reset to original
	createdJob.ValidFrom = time.Now().Add(-time.Hour)
	err = db.Save(createdJob).Error
	assert.Error(t, err, "Direct ValidFrom modification should fail")
	assert.Contains(t, err.Error(), "ValidFrom cannot be modified", "Error should mention ValidFrom protection")
}

// Test SoftDelete functionality
func TestSoftDelete(t *testing.T) {
	db := setupTestDB(t)

	// Create and update a job to have multiple versions
	job := &TestJob{
		Model:  Model{ID: "delete-test"},
		Status: "active",
	}

	_, err := CreateNew[*TestJob](db, job)
	require.NoError(t, err)

	_, err = Update[*TestJob](db, "delete-test", func(j *TestJob) {
		j.Status = "updated"
	})
	require.NoError(t, err)

	// Verify Latest returns the job before deletion
	var beforeDelete TestJob
	err = db.Scopes(Latest).Where("id = ?", "delete-test").First(&beforeDelete).Error
	require.NoError(t, err)
	assert.Equal(t, 2, beforeDelete.GetVersion())
	assert.Nil(t, beforeDelete.ValidTo)

	// Perform soft delete
	err = SoftDelete[*TestJob](db, "delete-test")
	require.NoError(t, err, "SoftDelete should succeed")

	// Verify Latest no longer returns the job
	var afterDelete TestJob
	err = db.Scopes(Latest).Where("id = ?", "delete-test").First(&afterDelete).Error
	assert.Error(t, err, "Latest should not find soft-deleted job")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "Should get record not found error")

	// Verify the previous latest version now has ValidTo set
	var deletedVersion TestJob
	err = db.Where("id = ? AND version = ?", "delete-test", 2).First(&deletedVersion).Error
	require.NoError(t, err)
	assert.NotNil(t, deletedVersion.ValidTo, "Soft-deleted version should have ValidTo set")

	// Verify all historical versions still exist
	var allVersions []TestJob
	err = db.Where("id = ?", "delete-test").Find(&allVersions).Error
	require.NoError(t, err)
	assert.Len(t, allVersions, 2, "All versions should still exist after soft delete")
}

// Test query scopes
func TestQueryScopes(t *testing.T) {
	db := setupTestDB(t)

	// Create job with multiple versions
	job := &TestJob{
		Model:  Model{ID: "scope-test"},
		Status: "active",
	}

	_, err := CreateNew[*TestJob](db, job)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond) // Small delay to ensure timestamp difference

	_, err = Update[*TestJob](db, "scope-test", func(j *TestJob) {
		j.Status = "updated"
	})
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = Update[*TestJob](db, "scope-test", func(j *TestJob) {
		j.Status = "final"
	})
	require.NoError(t, err)

	// Test Latest scope
	var latestJobs []TestJob
	err = db.Scopes(Latest).Where("id = ?", "scope-test").Find(&latestJobs).Error
	require.NoError(t, err)
	assert.Len(t, latestJobs, 1, "Latest should return 1 job")
	assert.Equal(t, 3, latestJobs[0].GetVersion())
	assert.Equal(t, "final", latestJobs[0].Status)

	// Test Historical scope
	var historicalJobs []TestJob
	err = db.Scopes(Historical).Where("id = ?", "scope-test").Find(&historicalJobs).Error
	require.NoError(t, err)
	assert.Len(t, historicalJobs, 2, "Historical should return 2 jobs")

	// Test AllVersions
	var allJobs []TestJob
	err = db.Scopes(AllVersions).Where("id = ?", "scope-test").Find(&allJobs).Error
	require.NoError(t, err)
	assert.Len(t, allJobs, 3, "AllVersions should return 3 jobs")

	// Test ByVersion scope
	var version2Jobs []TestJob
	err = db.Scopes(ByVersion(2)).Where("id = ?", "scope-test").Find(&version2Jobs).Error
	require.NoError(t, err)
	assert.Len(t, version2Jobs, 1, "ByVersion(2) should return 1 job")
	assert.Equal(t, 2, version2Jobs[0].GetVersion())
	assert.Equal(t, "updated", version2Jobs[0].Status)
}

// Test helper functions
func TestHelperFunctions(t *testing.T) {
	db := setupTestDB(t)

	// Create test job
	job := &TestJob{
		Model:  Model{ID: "helper-test"},
		Status: "active",
	}

	_, err := CreateNew[*TestJob](db, job)
	require.NoError(t, err)

	// Test Exists
	exists, err := Exists[*TestJob](db, "helper-test")
	require.NoError(t, err)
	assert.True(t, exists, "Job should exist")

	nonExists, err := Exists[*TestJob](db, "non-existent")
	require.NoError(t, err)
	assert.False(t, nonExists, "Non-existent job should not exist")

	// Test HasLatestVersion
	hasLatest, err := HasLatestVersion[*TestJob](db, "helper-test")
	require.NoError(t, err)
	assert.True(t, hasLatest, "Job should have latest version")

	// Test GetLatest
	latest, err := GetLatest[*TestJob](db, "helper-test")
	require.NoError(t, err)
	assert.Equal(t, 1, latest.GetVersion())

	// Test GetAllVersions
	versions, err := GetAllVersions[*TestJob](db, "helper-test")
	require.NoError(t, err)
	assert.Len(t, versions, 1)

	// Add another version and test again
	_, err = Update[*TestJob](db, "helper-test", func(j *TestJob) {
		j.Status = "updated"
	})
	require.NoError(t, err)

	versions, err = GetAllVersions[*TestJob](db, "helper-test")
	require.NoError(t, err)
	assert.Len(t, versions, 2)
	assert.Equal(t, 1, versions[0].GetVersion()) // Ordered oldest to newest
	assert.Equal(t, 2, versions[1].GetVersion())
}

// Test error cases
func TestErrorCases(t *testing.T) {
	db := setupTestDB(t)

	// Test updating non-existent entity
	_, err := Update[*TestJob](db, "non-existent", func(j *TestJob) {
		j.Status = "updated"
	})
	assert.Error(t, err, "Updating non-existent entity should fail")
	assert.Contains(t, err.Error(), "failed to find latest version", "Error should mention entity not found")

	// Test creating entity with empty business ID
	_, err = CreateNew[*TestJob](db, &TestJob{})
	assert.Error(t, err, "Creating entity with empty ID should fail")
	assert.Contains(t, err.Error(), "business ID is required", "Error should mention required business ID")

	// Test soft deleting non-existent entity
	err = SoftDelete[*TestJob](db, "non-existent")
	assert.Error(t, err, "Soft deleting non-existent entity should fail")

	// Test GetLatest on non-existent entity
	_, err = GetLatest[*TestJob](db, "non-existent")
	assert.Error(t, err, "Getting latest of non-existent entity should fail")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// Benchmark Update performance
func BenchmarkUpdate(b *testing.B) {
	db := setupTestDB(&testing.T{})

	// Create initial job
	job := &TestJob{
		Model:  Model{ID: "benchmark-job"},
		Status: "active",
		Rate:   50.0,
	}

	_, err := CreateNew[*TestJob](db, job)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Update[*TestJob](db, "benchmark-job", func(j *TestJob) {
			j.Rate = float64(50 + i)
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
