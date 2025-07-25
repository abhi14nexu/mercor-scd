package models

import (
	"github.com/abhi14nexu/mercor-scd/internal/scd"
)

// Job represents a job posting with SCD versioning capabilities
// Maps to the 'jobs' table in the database
type Job struct {
	scd.Model `gorm:"embedded"` // Embeds UID, ID, Version, ValidFrom, ValidTo

	// Business-specific fields
	Status       string  `gorm:"type:text;not null" json:"status" validate:"oneof=extended active paused completed"`
	Rate         float64 `gorm:"type:decimal(10,2);not null" json:"rate" validate:"gte=0"`
	Title        string  `gorm:"type:text;not null" json:"title" validate:"required,min=1,max=200"`
	CompanyID    string  `gorm:"type:text;not null" json:"company_id" validate:"required"`
	ContractorID string  `gorm:"type:text;not null" json:"contractor_id" validate:"required"`
}

// TableName specifies the table name for GORM
func (Job) TableName() string {
	return "jobs"
}

// NewJob creates a new Job with the given business ID and initial values
func NewJob(businessID, title, companyID, contractorID string, rate float64) *Job {
	return &Job{
		Model: scd.Model{
			ID: businessID,
		},
		Status:       "active",
		Rate:         rate,
		Title:        title,
		CompanyID:    companyID,
		ContractorID: contractorID,
	}
}

// IsActive returns true if the job is currently active
func (j *Job) IsActive() bool {
	return j.Status == "active"
}

// GetHourlyRate returns the hourly rate for this job
func (j *Job) GetHourlyRate() float64 {
	return j.Rate
}

// UpdateRate updates the job's hourly rate
func (j *Job) UpdateRate(newRate float64) {
	j.Rate = newRate
}

// Pause changes the job status to paused
func (j *Job) Pause() {
	j.Status = "paused"
}

// Resume changes the job status to active
func (j *Job) Resume() {
	j.Status = "active"
}

// Complete marks the job as completed
func (j *Job) Complete() {
	j.Status = "completed"
}
