package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/abhi14nexu/mercor-scd/internal/models"
	"github.com/abhi14nexu/mercor-scd/internal/scd"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// seedCmd represents the seed command
var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Populate the database with sample data",
	Long: `Seeds the database with sample data to demonstrate SCD functionality:

- Creates 10 jobs, each with 3 versions showing status and rate changes
- Creates 40 timelogs referencing various job versions
- Creates 40 payment line items based on the timelogs
- Uses realistic company and contractor IDs for querying`,
	Run: runSeed,
}

func runSeed(cmd *cobra.Command, args []string) {
	log.Println("🌱 Starting database seeding...")

	// Seed data for consistent demo
	rand.Seed(time.Now().UnixNano())

	companies := []string{"company-acme", "company-tech", "company-startup", "company-corp"}
	contractors := []string{"contractor-alice", "contractor-bob", "contractor-carol", "contractor-dave"}
	jobTitles := []string{
		"Software Engineer", "Frontend Developer", "Backend Developer",
		"DevOps Engineer", "Data Scientist", "Product Manager",
		"UI/UX Designer", "QA Engineer", "Tech Lead", "Full Stack Developer",
	}

	statuses := []string{"active", "paused", "completed"}

	// Step 1: Create 10 jobs with 3 versions each
	log.Println("📋 Creating jobs with version history...")

	var jobUIDs []uuid.UUID // Track job UIDs for timelog creation

	for i := 1; i <= 10; i++ {
		jobID := fmt.Sprintf("job-%d", i)

		// Create initial job version
		job := models.NewJob(
			jobID,
			jobTitles[i-1],
			companies[rand.Intn(len(companies))],
			contractors[rand.Intn(len(contractors))],
			float64(40+rand.Intn(60)), // Rate between $40-100
		)

		createdJob, err := scd.CreateNew[*models.Job](db, job)
		if err != nil {
			log.Fatalf("Failed to create job %s: %v", jobID, err)
		}

		currentJob := createdJob
		jobUIDs = append(jobUIDs, currentJob.GetUID())

		// Create 2 additional versions with changes
		for version := 2; version <= 3; version++ {
			time.Sleep(10 * time.Millisecond) // Small delay for distinct timestamps

			updatedJob, err := scd.Update[*models.Job](db, jobID, func(j *models.Job) {
				// Vary status and rate changes
				if version == 2 {
					// Version 2: Change rate
					j.Rate = j.Rate + float64(5+rand.Intn(15)) // Increase rate
				} else {
					// Version 3: Change status
					j.Status = statuses[rand.Intn(len(statuses))]
					if rand.Float32() < 0.3 { // 30% chance to also change rate
						j.Rate = j.Rate + float64(-5+rand.Intn(11)) // ±5 rate change
					}
				}
			})

			if err != nil {
				log.Fatalf("Failed to update job %s to version %d: %v", jobID, version, err)
			}

			currentJob = updatedJob
			jobUIDs = append(jobUIDs, currentJob.GetUID())
		}

		log.Printf("✅ Created job %s with 3 versions (latest: %s, rate: $%.2f)",
			jobID, currentJob.Status, currentJob.Rate)
	}

	// Step 2: Create 40 timelogs
	log.Println("⏰ Creating timelogs...")

	var timelogUIDs []uuid.UUID

	for i := 1; i <= 40; i++ {
		timelogID := fmt.Sprintf("timelog-%d", i)

		// Random job version to reference
		jobUID := jobUIDs[rand.Intn(len(jobUIDs))]

		// Random time range (last 30 days)
		daysAgo := rand.Intn(30)
		startTime := time.Now().AddDate(0, 0, -daysAgo).Add(time.Duration(rand.Intn(8)) * time.Hour)
		duration := time.Duration(1+rand.Intn(8)) * time.Hour // 1-8 hours
		endTime := startTime.Add(duration)

		timelog := models.NewTimelog(timelogID, jobUID, startTime, endTime)

		// Add some variety in timelog types
		if rand.Float32() < 0.2 { // 20% adjusted
			timelog.Type = "adjusted"
		}

		createdTimelog, err := scd.CreateNew[*models.Timelog](db, timelog)
		if err != nil {
			log.Fatalf("Failed to create timelog %s: %v", timelogID, err)
		}

		timelogUIDs = append(timelogUIDs, createdTimelog.GetUID())

		// Randomly create an adjusted version for some timelogs
		if rand.Float32() < 0.15 { // 15% get adjustments
			time.Sleep(5 * time.Millisecond)

			_, err := scd.Update[*models.Timelog](db, timelogID, func(t *models.Timelog) {
				// Adjust duration by ±30 minutes
				adjustment := time.Duration(-30+rand.Intn(61)) * time.Minute
				newEndTime := time.Unix(t.TimeEnd, 0).Add(adjustment)
				t.AdjustTimes(time.Unix(t.TimeStart, 0), newEndTime)
			})

			if err != nil {
				log.Printf("Warning: Failed to create adjustment for timelog %s: %v", timelogID, err)
			}
		}

		if i%10 == 0 {
			log.Printf("✅ Created %d timelogs", i)
		}
	}

	// Step 3: Create 40 payment line items
	log.Println("💰 Creating payment line items...")

	paymentStatuses := []string{"not-paid", "paid", "failed"}

	for i := 1; i <= 40; i++ {
		paymentID := fmt.Sprintf("payment-%d", i)

		// Random job and timelog to reference
		jobUID := jobUIDs[rand.Intn(len(jobUIDs))]
		timelogUID := timelogUIDs[rand.Intn(len(timelogUIDs))]

		// Calculate realistic amount (rate * hours)
		amount := float64(50+rand.Intn(50)) * (1.0 + rand.Float64()*7.0) // $50-100 rate * 1-8 hours

		payment := models.NewPaymentLineItem(paymentID, jobUID, timelogUID, amount)

		// Set realistic payment status distribution
		statusRand := rand.Float32()
		if statusRand < 0.7 { // 70% paid
			payment.Status = "paid"
		} else if statusRand < 0.9 { // 20% not-paid
			payment.Status = "not-paid"
		} else { // 10% failed
			payment.Status = "failed"
		}

		_, err := scd.CreateNew[*models.PaymentLineItem](db, payment)
		if err != nil {
			log.Fatalf("Failed to create payment %s: %v", paymentID, err)
		}

		// Some payments get status updates
		if rand.Float32() < 0.2 { // 20% get status changes
			time.Sleep(5 * time.Millisecond)

			_, err := scd.Update[*models.PaymentLineItem](db, paymentID, func(p *models.PaymentLineItem) {
				// Change status (e.g., not-paid -> paid)
				newStatus := paymentStatuses[rand.Intn(len(paymentStatuses))]
				p.Status = newStatus
			})

			if err != nil {
				log.Printf("Warning: Failed to update payment status for %s: %v", paymentID, err)
			}
		}

		if i%10 == 0 {
			log.Printf("✅ Created %d payment line items", i)
		}
	}

	log.Println("🎉 Database seeding completed successfully!")
	log.Println("📊 Summary:")
	log.Println("   • 10 jobs (30 total versions)")
	log.Println("   • 40+ timelogs (some with adjustments)")
	log.Println("   • 40+ payment line items (some with status updates)")
	log.Println("")
	log.Printf("💡 Try: go run cmd/demo latest-jobs --company=%s\n", companies[0])
	log.Printf("💡 Try: go run cmd/demo payments --contractor=%s\n", contractors[0])
}
