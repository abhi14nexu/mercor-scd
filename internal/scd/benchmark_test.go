package scd

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// BenchmarkDB creates a database optimized for benchmarking
func setupBenchmarkDB(b *testing.B) *gorm.DB {
	b.Helper()

	// Use WAL mode and optimize for performance
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000000"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Disable logging for cleaner benchmark output
	})
	if err != nil {
		b.Fatalf("Failed to connect to benchmark database: %v", err)
	}

	// Auto-migrate test models
	err = db.AutoMigrate(&ModelTestJob{}, &ModelTestTimelog{}, &ModelTestPaymentLineItem{})
	if err != nil {
		b.Fatalf("Failed to auto-migrate benchmark models: %v", err)
	}

	return db
}

// seedBenchmarkData populates the database with test data for realistic benchmarks
func seedBenchmarkData(db *gorm.DB, numJobs int) error {
	jobs := make([]*ModelTestJob, numJobs)

	// Create initial versions
	for i := 0; i < numJobs; i++ {
		job := &ModelTestJob{
			SQLiteModel: SQLiteModel{
				ID: fmt.Sprintf("bench-job-%d", i),
			},
			Status:       "active",
			Rate:         100.0 + float64(i%50), // Varying rates
			Title:        fmt.Sprintf("Benchmark Job %d", i),
			CompanyID:    fmt.Sprintf("company-%d", i%10),     // 10 companies
			ContractorID: fmt.Sprintf("contractor-%d", i%100), // 100 contractors
		}

		created, err := CreateNew(db, job)
		if err != nil {
			return fmt.Errorf("failed to create job %d: %w", i, err)
		}
		jobs[i] = created
	}

	// Create some historical versions (update 20% of jobs to have 2-3 versions each)
	updateCount := numJobs / 5
	for i := 0; i < updateCount; i++ {
		jobID := jobs[i].GetBusinessID()

		// Create version 2
		_, err := Update(db, jobID, func(j *ModelTestJob) {
			j.Rate += 10.0
			j.Title += " - Updated"
		})
		if err != nil {
			return fmt.Errorf("failed to update job %s v2: %w", jobID, err)
		}

		// Create version 3 for half of updated jobs
		if i%2 == 0 {
			_, err := Update(db, jobID, func(j *ModelTestJob) {
				j.Rate += 5.0
				j.Status = "paused"
			})
			if err != nil {
				return fmt.Errorf("failed to update job %s v3: %w", jobID, err)
			}
		}
	}

	return nil
}

// BenchmarkCreateNew measures the performance of creating new SCD records
func BenchmarkCreateNew(b *testing.B) {
	db := setupBenchmarkDB(b)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		job := &ModelTestJob{
			SQLiteModel: SQLiteModel{
				ID: fmt.Sprintf("create-bench-%d", i),
			},
			Status:       "active",
			Rate:         100.0,
			Title:        fmt.Sprintf("Benchmark Create Job %d", i),
			CompanyID:    "bench-company",
			ContractorID: "bench-contractor",
		}

		_, err := CreateNew(db, job)
		if err != nil {
			b.Fatalf("CreateNew failed: %v", err)
		}
	}
}

// BenchmarkSCDUpdate measures the performance of SCD updates
func BenchmarkSCDUpdate(b *testing.B) {
	db := setupBenchmarkDB(b)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	// Seed with initial data
	const seedJobs = 1000
	err := seedBenchmarkData(db, seedJobs)
	if err != nil {
		b.Fatalf("Failed to seed data: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		jobID := fmt.Sprintf("bench-job-%d", i%seedJobs)

		_, err := Update(db, jobID, func(j *ModelTestJob) {
			j.Rate += 1.0
			j.Title = fmt.Sprintf("Updated Job %d - iteration %d", i%seedJobs, i)
		})
		if err != nil {
			b.Fatalf("Update failed: %v", err)
		}
	}
}

// BenchmarkGetLatest measures the performance of retrieving current records
func BenchmarkGetLatest(b *testing.B) {
	db := setupBenchmarkDB(b)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	// Seed with data including historical versions
	const seedJobs = 5000
	err := seedBenchmarkData(db, seedJobs)
	if err != nil {
		b.Fatalf("Failed to seed data: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		jobID := fmt.Sprintf("bench-job-%d", i%seedJobs)

		_, err := GetLatest[*ModelTestJob](db, jobID)
		if err != nil {
			b.Fatalf("GetLatest failed: %v", err)
		}
	}
}

// BenchmarkAsOf measures the performance of point-in-time queries
func BenchmarkAsOf(b *testing.B) {
	db := setupBenchmarkDB(b)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	// Seed with data
	const seedJobs = 2000
	err := seedBenchmarkData(db, seedJobs)
	if err != nil {
		b.Fatalf("Failed to seed data: %v", err)
	}

	// Capture a timestamp for AsOf queries
	asOfTime := time.Now().Add(-5 * time.Minute)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		jobID := fmt.Sprintf("bench-job-%d", i%seedJobs)

		var job ModelTestJob
		err := db.Scopes(AsOf(asOfTime)).Where("id = ?", jobID).First(&job).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			b.Fatalf("AsOf query failed: %v", err)
		}
	}
}

// BenchmarkBulkUpdates measures performance of bulk update operations
func BenchmarkBulkUpdates(b *testing.B) {
	sizes := []int{10, 50, 100, 500}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("BulkSize-%d", size), func(b *testing.B) {
			db := setupBenchmarkDB(b)
			defer func() {
				if sqlDB, err := db.DB(); err == nil {
					sqlDB.Close()
				}
			}()

			// Seed initial data
			err := seedBenchmarkData(db, size*2) // Ensure we have enough jobs
			if err != nil {
				b.Fatalf("Failed to seed data: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Update a batch of jobs
				for j := 0; j < size; j++ {
					jobID := fmt.Sprintf("bench-job-%d", (i*size+j)%(size*2))

					_, err := Update(db, jobID, func(job *ModelTestJob) {
						job.Rate += 0.5
						job.Title = fmt.Sprintf("Bulk Updated %d-%d", i, j)
					})
					if err != nil {
						b.Fatalf("Bulk update failed: %v", err)
					}
				}
			}
		})
	}
}

// BenchmarkConcurrentUpdates measures performance under concurrent load
func BenchmarkConcurrentUpdates(b *testing.B) {
	concurrencyLevels := []int{1, 2, 4, 8}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			db := setupBenchmarkDB(b)
			defer func() {
				if sqlDB, err := db.DB(); err == nil {
					sqlDB.Close()
				}
			}()

			// Seed with enough jobs for concurrent access
			const seedJobs = 1000
			err := seedBenchmarkData(db, seedJobs)
			if err != nil {
				b.Fatalf("Failed to seed data: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					jobID := fmt.Sprintf("bench-job-%d", i%seedJobs)

					_, err := Update(db, jobID, func(job *ModelTestJob) {
						job.Rate += 0.1
					})
					// Ignore database locked errors for SQLite under high concurrency
					if err != nil {
						errStr := err.Error()
						if !strings.Contains(errStr, "database table is locked") &&
							!strings.Contains(errStr, "database is locked") {
							b.Fatalf("Concurrent update failed: %v", err)
						}
						// SQLite table locking is expected under high concurrency
					}
					i++
				}
			})
		})
	}
}

// BenchmarkQueryComplexity measures performance with different data sizes
func BenchmarkQueryComplexity(b *testing.B) {
	dataSizes := []int{1000, 5000, 10000, 25000}

	for _, size := range dataSizes {
		b.Run(fmt.Sprintf("DataSize-%d", size), func(b *testing.B) {
			db := setupBenchmarkDB(b)
			defer func() {
				if sqlDB, err := db.DB(); err == nil {
					sqlDB.Close()
				}
			}()

			err := seedBenchmarkData(db, size)
			if err != nil {
				b.Fatalf("Failed to seed data: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				jobID := fmt.Sprintf("bench-job-%d", i%size)

				_, err := GetLatest[*ModelTestJob](db, jobID)
				if err != nil {
					b.Fatalf("GetLatest failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkHistoricalQueries measures performance of historical data access
func BenchmarkHistoricalQueries(b *testing.B) {
	db := setupBenchmarkDB(b)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	// Create jobs with many versions to test historical query performance
	const numJobs = 500
	const versionsPerJob = 10

	for i := 0; i < numJobs; i++ {
		jobID := fmt.Sprintf("hist-job-%d", i)

		// Create initial version
		job := &ModelTestJob{
			SQLiteModel: SQLiteModel{
				ID: jobID,
			},
			Status:       "active",
			Rate:         100.0,
			Title:        fmt.Sprintf("Historical Job %d", i),
			CompanyID:    "hist-company",
			ContractorID: "hist-contractor",
		}

		_, err := CreateNew(db, job)
		if err != nil {
			b.Fatalf("Failed to create job: %v", err)
		}

		// Create multiple versions
		for v := 1; v < versionsPerJob; v++ {
			_, err := Update(db, jobID, func(j *ModelTestJob) {
				j.Rate += float64(v)
				j.Title = fmt.Sprintf("Historical Job %d v%d", i, v+1)
			})
			if err != nil {
				b.Fatalf("Failed to update job: %v", err)
			}
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		jobID := fmt.Sprintf("hist-job-%d", i%numJobs)

		// Query all versions of this job
		var versions []ModelTestJob
		err := db.Where("id = ?", jobID).Order("version ASC").Find(&versions).Error
		if err != nil {
			b.Fatalf("Historical query failed: %v", err)
		}
	}
}

// BenchmarkMemoryUsage provides insight into memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	db := setupBenchmarkDB(b)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create, update, and query in a single benchmark iteration
		job := &ModelTestJob{
			SQLiteModel: SQLiteModel{
				ID: fmt.Sprintf("memory-job-%d", i),
			},
			Status:       "active",
			Rate:         100.0,
			Title:        "Memory Test Job",
			CompanyID:    "memory-company",
			ContractorID: "memory-contractor",
		}

		// Create
		created, err := CreateNew(db, job)
		if err != nil {
			b.Fatalf("Create failed: %v", err)
		}

		// Update
		_, err = Update(db, created.GetBusinessID(), func(j *ModelTestJob) {
			j.Rate += 10.0
		})
		if err != nil {
			b.Fatalf("Update failed: %v", err)
		}

		// Query
		_, err = GetLatest[*ModelTestJob](db, created.GetBusinessID())
		if err != nil {
			b.Fatalf("GetLatest failed: %v", err)
		}
	}
}
