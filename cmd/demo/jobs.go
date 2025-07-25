package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/abhi14nexu/mercor-scd/internal/models"
	"github.com/abhi14nexu/mercor-scd/internal/scd"
	"github.com/spf13/cobra"
)

// latestJobsCmd represents the latest-jobs command
var latestJobsCmd = &cobra.Command{
	Use:   "latest-jobs",
	Short: "Query latest versions of jobs by company",
	Long: `Retrieves the latest versions of all jobs belonging to a specific company.

This command demonstrates the use of the scd.Latest scope to filter for only
the current/active versions of jobs, avoiding the need to manually add
'WHERE valid_to IS NULL' conditions.

Example:
  demo latest-jobs --company=company-acme`,
	Run: runLatestJobs,
}

var (
	companyFlag string
)

func init() {
	// Add required company flag
	latestJobsCmd.Flags().StringVar(&companyFlag, "company", "", "Company ID to filter jobs (required)")
	latestJobsCmd.MarkFlagRequired("company")
}

func runLatestJobs(cmd *cobra.Command, args []string) {
	fmt.Fprintf(os.Stderr, "🔍 Querying latest jobs for company: %s\n", companyFlag)

	// Use SCD scope to get only latest versions
	var jobs []models.Job
	result := db.Scopes(scd.Latest).
		Where("company_id = ?", companyFlag).
		Order("id ASC"). // Order by business ID for consistent output
		Find(&jobs)

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "Failed to query jobs: %v\n", result.Error)
		os.Exit(1)
	}

	if len(jobs) == 0 {
		fmt.Fprintf(os.Stderr, "⚠️  No jobs found for company: %s\n", companyFlag)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ Found %d latest job(s) for company: %s\n", len(jobs), companyFlag)

	// Create output structure with additional metadata
	type JobOutput struct {
		BusinessID   string  `json:"business_id"`
		UID          string  `json:"uid"`
		Version      int     `json:"version"`
		Status       string  `json:"status"`
		Rate         float64 `json:"rate"`
		Title        string  `json:"title"`
		CompanyID    string  `json:"company_id"`
		ContractorID string  `json:"contractor_id"`
		ValidFrom    string  `json:"valid_from"`
		ValidTo      *string `json:"valid_to"`
	}

	var output []JobOutput
	for _, job := range jobs {
		jobOut := JobOutput{
			BusinessID:   job.GetBusinessID(),
			UID:          job.GetUID().String(),
			Version:      job.GetVersion(),
			Status:       job.Status,
			Rate:         job.Rate,
			Title:        job.Title,
			CompanyID:    job.CompanyID,
			ContractorID: job.ContractorID,
			ValidFrom:    job.ValidFrom.Format("2006-01-02 15:04:05"),
		}

		if job.ValidTo != nil {
			validToStr := job.ValidTo.Format("2006-01-02 15:04:05")
			jobOut.ValidTo = &validToStr
		}

		output = append(output, jobOut)
	}

	// Pretty-print JSON output
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\n📋 Latest Jobs for Company: %s\n", companyFlag)
	fmt.Fprintf(os.Stderr, "════════════════════════════════════════\n")
	fmt.Println(string(jsonData))

	// Additional summary
	fmt.Fprintf(os.Stderr, "\n📊 Summary:\n")
	fmt.Fprintf(os.Stderr, "• Total jobs found: %d\n", len(jobs))

	// Status breakdown
	statusCount := make(map[string]int)
	var totalRate float64
	for _, job := range jobs {
		statusCount[job.Status]++
		totalRate += job.Rate
	}

	fmt.Fprintf(os.Stderr, "• Average rate: $%.2f\n", totalRate/float64(len(jobs)))
	fmt.Fprintf(os.Stderr, "• Status breakdown:\n")
	for status, count := range statusCount {
		fmt.Fprintf(os.Stderr, "  - %s: %d\n", status, count)
	}

	fmt.Fprintf(os.Stderr, "\n💡 To see version history: SELECT * FROM jobs WHERE company_id='%s' ORDER BY id, version;\n", companyFlag)
}
