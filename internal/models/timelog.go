package models

import (
	"time"

	"github.com/abhi14nexu/mercor-scd/internal/scd"
	"github.com/google/uuid"
)

// Timelog represents a time tracking entry with SCD versioning capabilities
// Maps to the 'timelogs' table in the database
type Timelog struct {
	scd.Model `gorm:"embedded"` // Embeds UID, ID, Version, ValidFrom, ValidTo

	// Business-specific fields
	Duration  int64     `gorm:"type:bigint;not null" json:"duration" validate:"gte=0"`                      // milliseconds
	TimeStart int64     `gorm:"type:bigint;not null" json:"time_start" validate:"required"`                 // Unix timestamp
	TimeEnd   int64     `gorm:"type:bigint;not null" json:"time_end" validate:"required,gtfield=TimeStart"` // Unix timestamp
	Type      string    `gorm:"type:text;not null" json:"type" validate:"oneof=captured adjusted"`          // captured, adjusted
	JobUID    uuid.UUID `gorm:"type:uuid;not null" json:"job_uid" validate:"required"`                      // FK to specific job version
}

// TableName specifies the table name for GORM
func (Timelog) TableName() string {
	return "timelogs"
}

// NewTimelog creates a new Timelog with the given business ID and initial values
func NewTimelog(businessID string, jobUID uuid.UUID, startTime, endTime time.Time) *Timelog {
	start := startTime.Unix()
	end := endTime.Unix()
	duration := end - start

	return &Timelog{
		Model: scd.Model{
			ID: businessID,
		},
		Duration:  duration * 1000, // Convert to milliseconds
		TimeStart: start,
		TimeEnd:   end,
		Type:      "captured",
		JobUID:    jobUID,
	}
}

// NewCapturedTimelog creates a new captured timelog entry
func NewCapturedTimelog(businessID string, jobUID uuid.UUID, startTime, endTime time.Time) *Timelog {
	timelog := NewTimelog(businessID, jobUID, startTime, endTime)
	timelog.Type = "captured"
	return timelog
}

// NewAdjustedTimelog creates a new adjusted timelog entry
func NewAdjustedTimelog(businessID string, jobUID uuid.UUID, startTime, endTime time.Time) *Timelog {
	timelog := NewTimelog(businessID, jobUID, startTime, endTime)
	timelog.Type = "adjusted"
	return timelog
}

// GetDurationHours returns the duration in hours as a float64
func (t *Timelog) GetDurationHours() float64 {
	return float64(t.Duration) / (1000 * 60 * 60) // Convert milliseconds to hours
}

// GetDurationMinutes returns the duration in minutes
func (t *Timelog) GetDurationMinutes() int64 {
	return t.Duration / (1000 * 60) // Convert milliseconds to minutes
}

// GetStartTime returns the start time as a time.Time
func (t *Timelog) GetStartTime() time.Time {
	return time.Unix(t.TimeStart, 0)
}

// GetEndTime returns the end time as a time.Time
func (t *Timelog) GetEndTime() time.Time {
	return time.Unix(t.TimeEnd, 0)
}

// IsCaptured returns true if this is a captured timelog
func (t *Timelog) IsCaptured() bool {
	return t.Type == "captured"
}

// IsAdjusted returns true if this is an adjusted timelog
func (t *Timelog) IsAdjusted() bool {
	return t.Type == "adjusted"
}

// UpdateDuration updates the timelog duration and end time
func (t *Timelog) UpdateDuration(newDurationMinutes int64) {
	t.Duration = newDurationMinutes * 60 * 1000         // Convert minutes to milliseconds
	t.TimeEnd = t.TimeStart + (newDurationMinutes * 60) // Update end time
	t.Type = "adjusted"                                 // Mark as adjusted when duration is changed
}

// AdjustTimes updates both start and end times
func (t *Timelog) AdjustTimes(startTime, endTime time.Time) {
	t.TimeStart = startTime.Unix()
	t.TimeEnd = endTime.Unix()
	t.Duration = (t.TimeEnd - t.TimeStart) * 1000 // Convert to milliseconds
	t.Type = "adjusted"
}
