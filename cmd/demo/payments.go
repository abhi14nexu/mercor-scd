package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/abhi14nexu/mercor-scd/internal/models"
	"github.com/abhi14nexu/mercor-scd/internal/scd"
	"github.com/spf13/cobra"
)

// paymentsCmd represents the payments command
var paymentsCmd = &cobra.Command{
	Use:   "payments",
	Short: "Query payment line items by contractor",
	Long: `Finds all job versions associated with a contractor, then retrieves the latest
versions of all related payment line items.

This command demonstrates:
- Querying across multiple SCD entities with relationships
- Using scd.Latest scope for multiple related tables
- Complex queries involving foreign key relationships between versioned entities

Example:
  demo payments --contractor=contractor-alice`,
	Run: runPayments,
}

var (
	contractorFlag string
)

func init() {
	// Add required contractor flag
	paymentsCmd.Flags().StringVar(&contractorFlag, "contractor", "", "Contractor ID to filter payments (required)")
	paymentsCmd.MarkFlagRequired("contractor")
}

func runPayments(cmd *cobra.Command, args []string) {
	fmt.Fprintf(os.Stderr, "💰 Querying payments for contractor: %s\n", contractorFlag)

	// Step 1: Find all job versions for this contractor
	// We need ALL versions because payments might reference any version
	var jobVersions []models.Job
	result := db.Where("contractor_id = ?", contractorFlag).
		Order("id ASC, version ASC").
		Find(&jobVersions)

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "Failed to query job versions: %v\n", result.Error)
		os.Exit(1)
	}

	if len(jobVersions) == 0 {
		fmt.Fprintf(os.Stderr, "⚠️  No jobs found for contractor: %s\n", contractorFlag)
		return
	}

	fmt.Fprintf(os.Stderr, "📋 Found %d job version(s) for contractor: %s\n", len(jobVersions), contractorFlag)

	// Step 2: Get UIDs of all job versions
	var jobUIDs []string
	jobUIDToInfo := make(map[string]models.Job)

	for _, job := range jobVersions {
		uidStr := job.GetUID().String()
		jobUIDs = append(jobUIDs, uidStr)
		jobUIDToInfo[uidStr] = job
	}

	// Step 3: Find latest payment line items that reference these job versions
	var payments []models.PaymentLineItem
	result = db.Scopes(scd.Latest).
		Where("job_uid IN ?", jobUIDs).
		Order("id ASC").
		Find(&payments)

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "Failed to query payment line items: %v\n", result.Error)
		os.Exit(1)
	}

	if len(payments) == 0 {
		fmt.Fprintf(os.Stderr, "⚠️  No payment line items found for contractor: %s\n", contractorFlag)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ Found %d latest payment line item(s) for contractor: %s\n", len(payments), contractorFlag)

	// Create enhanced output structure with job information
	type PaymentOutput struct {
		PaymentInfo struct {
			BusinessID string  `json:"business_id"`
			UID        string  `json:"uid"`
			Version    int     `json:"version"`
			Amount     float64 `json:"amount"`
			Status     string  `json:"status"`
			ValidFrom  string  `json:"valid_from"`
			ValidTo    *string `json:"valid_to"`
		} `json:"payment_info"`
		RelatedJob struct {
			BusinessID string  `json:"business_id"`
			UID        string  `json:"uid"`
			Version    int     `json:"version"`
			Title      string  `json:"title"`
			Rate       float64 `json:"rate"`
			Status     string  `json:"status"`
		} `json:"related_job"`
		TimelogUID string `json:"timelog_uid"`
	}

	var output []PaymentOutput
	var totalAmount float64

	for _, payment := range payments {
		// Get related job information
		jobUID := payment.JobUID.String()
		relatedJob, exists := jobUIDToInfo[jobUID]
		if !exists {
			fmt.Fprintf(os.Stderr, "Warning: Could not find job info for UID %s\n", jobUID)
			continue
		}

		paymentOut := PaymentOutput{
			TimelogUID: payment.TimelogUID.String(),
		}

		// Payment info
		paymentOut.PaymentInfo.BusinessID = payment.GetBusinessID()
		paymentOut.PaymentInfo.UID = payment.GetUID().String()
		paymentOut.PaymentInfo.Version = payment.GetVersion()
		paymentOut.PaymentInfo.Amount = payment.Amount
		paymentOut.PaymentInfo.Status = payment.Status
		paymentOut.PaymentInfo.ValidFrom = payment.ValidFrom.Format("2006-01-02 15:04:05")

		if payment.ValidTo != nil {
			validToStr := payment.ValidTo.Format("2006-01-02 15:04:05")
			paymentOut.PaymentInfo.ValidTo = &validToStr
		}

		// Related job info
		paymentOut.RelatedJob.BusinessID = relatedJob.GetBusinessID()
		paymentOut.RelatedJob.UID = relatedJob.GetUID().String()
		paymentOut.RelatedJob.Version = relatedJob.GetVersion()
		paymentOut.RelatedJob.Title = relatedJob.Title
		paymentOut.RelatedJob.Rate = relatedJob.Rate
		paymentOut.RelatedJob.Status = relatedJob.Status

		output = append(output, paymentOut)
		totalAmount += payment.Amount
	}

	// Pretty-print JSON output
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\n💰 Payment Line Items for Contractor: %s\n", contractorFlag)
	fmt.Fprintf(os.Stderr, "═══════════════════════════════════════════════\n")
	fmt.Println(string(jsonData))

	// Additional summary
	fmt.Fprintf(os.Stderr, "\n📊 Summary:\n")
	fmt.Fprintf(os.Stderr, "• Total payment line items: %d\n", len(payments))
	fmt.Fprintf(os.Stderr, "• Total amount: $%.2f\n", totalAmount)
	fmt.Fprintf(os.Stderr, "• Average amount: $%.2f\n", totalAmount/float64(len(payments)))

	// Status breakdown
	statusCount := make(map[string]int)
	statusAmount := make(map[string]float64)

	for _, payment := range payments {
		statusCount[payment.Status]++
		statusAmount[payment.Status] += payment.Amount
	}

	fmt.Fprintf(os.Stderr, "• Payment status breakdown:\n")
	for status, count := range statusCount {
		fmt.Fprintf(os.Stderr, "  - %s: %d items ($%.2f total)\n", status, count, statusAmount[status])
	}

	// Job information
	latestJobs := make(map[string]models.Job)
	for _, job := range jobVersions {
		existing, exists := latestJobs[job.GetBusinessID()]
		if !exists || job.GetVersion() > existing.GetVersion() {
			latestJobs[job.GetBusinessID()] = job
		}
	}

	fmt.Fprintf(os.Stderr, "• Related jobs (latest versions): %d\n", len(latestJobs))
	for _, job := range latestJobs {
		fmt.Fprintf(os.Stderr, "  - %s: %s ($%.2f/hr, %s)\n",
			job.GetBusinessID(), job.Title, job.Rate, job.Status)
	}

	fmt.Fprintf(os.Stderr, "\n💡 To see all versions: SELECT * FROM payment_line_items WHERE job_uid IN (SELECT uid FROM jobs WHERE contractor_id='%s');\n", contractorFlag)
}
