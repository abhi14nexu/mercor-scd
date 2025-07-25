package models

import (
	"fmt"

	"github.com/abhi14nexu/mercor-scd/internal/scd"
	"github.com/google/uuid"
)

// PaymentLineItem represents a payment calculation with SCD versioning capabilities
// Maps to the 'payment_line_items' table in the database
type PaymentLineItem struct {
	scd.Model `gorm:"embedded"` // Embeds UID, ID, Version, ValidFrom, ValidTo

	// Business-specific fields
	JobUID     uuid.UUID `gorm:"type:uuid;not null" json:"job_uid" validate:"required"`                  // FK to specific job version
	TimelogUID uuid.UUID `gorm:"type:uuid;not null" json:"timelog_uid" validate:"required"`              // FK to specific timelog version
	Amount     float64   `gorm:"type:decimal(10,2);not null" json:"amount" validate:"gte=0"`             // calculated payment amount
	Status     string    `gorm:"type:text;not null" json:"status" validate:"oneof=not-paid paid failed"` // not-paid, paid, failed
}

// TableName specifies the table name for GORM
func (PaymentLineItem) TableName() string {
	return "payment_line_items"
}

// NewPaymentLineItem creates a new PaymentLineItem with the given business ID and calculation details
func NewPaymentLineItem(businessID string, jobUID, timelogUID uuid.UUID, amount float64) *PaymentLineItem {
	return &PaymentLineItem{
		Model: scd.Model{
			ID: businessID,
		},
		JobUID:     jobUID,
		TimelogUID: timelogUID,
		Amount:     amount,
		Status:     "not-paid",
	}
}

// CalculateAmount calculates the payment amount based on job rate and timelog duration
func CalculateAmount(job *Job, timelog *Timelog) float64 {
	hours := timelog.GetDurationHours()
	return job.Rate * hours
}

// NewCalculatedPaymentLineItem creates a new PaymentLineItem with auto-calculated amount
func NewCalculatedPaymentLineItem(businessID string, job *Job, timelog *Timelog) *PaymentLineItem {
	amount := CalculateAmount(job, timelog)

	return &PaymentLineItem{
		Model: scd.Model{
			ID: businessID,
		},
		JobUID:     job.GetUID(),
		TimelogUID: timelog.GetUID(),
		Amount:     amount,
		Status:     "not-paid",
	}
}

// IsNotPaid returns true if payment is pending
func (p *PaymentLineItem) IsNotPaid() bool {
	return p.Status == "not-paid"
}

// IsPaid returns true if payment has been completed
func (p *PaymentLineItem) IsPaid() bool {
	return p.Status == "paid"
}

// IsFailed returns true if payment failed
func (p *PaymentLineItem) IsFailed() bool {
	return p.Status == "failed"
}

// MarkPaid marks the payment as successfully paid
func (p *PaymentLineItem) MarkPaid() {
	p.Status = "paid"
}

// MarkFailed marks the payment as failed
func (p *PaymentLineItem) MarkFailed() {
	p.Status = "failed"
}

// MarkNotPaid resets the payment to not-paid status
func (p *PaymentLineItem) MarkNotPaid() {
	p.Status = "not-paid"
}

// UpdateAmount updates the payment amount (useful for adjustments)
func (p *PaymentLineItem) UpdateAmount(newAmount float64) {
	p.Amount = newAmount
}

// GetAmountCents returns the amount in cents for precise currency handling
func (p *PaymentLineItem) GetAmountCents() int64 {
	return int64(p.Amount * 100)
}

// GetFormattedAmount returns the amount formatted as a currency string
func (p *PaymentLineItem) GetFormattedAmount() string {
	return fmt.Sprintf("$%.2f", p.Amount)
}
