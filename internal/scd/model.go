package scd

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SCDModel interface ensures models have required SCD methods
type SCDModel interface {
	GetUID() uuid.UUID
	GetBusinessID() string
	GetVersion() int
	SetUID(uuid.UUID)
	SetBusinessID(string)
	SetVersion(int)
}

// Model provides SCD functionality when embedded in domain models
type Model struct {
	UID       uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"uid"`
	ID        string     `gorm:"index:idx_id_ver,unique;not null" json:"id"`
	Version   int        `gorm:"index:idx_id_ver,unique;not null" json:"version"`
	ValidFrom time.Time  `gorm:"autoCreateTime:nano;not null" json:"valid_from"`
	ValidTo   *time.Time `gorm:"index:idx_latest,where:valid_to IS NULL" json:"valid_to,omitempty"`
}

// GetUID returns the UUID primary key
func (m *Model) GetUID() uuid.UUID {
	return m.UID
}

// GetBusinessID returns the business identifier
func (m *Model) GetBusinessID() string {
	return m.ID
}

// GetVersion returns the version number
func (m *Model) GetVersion() int {
	return m.Version
}

// SetUID sets the UUID primary key
func (m *Model) SetUID(uid uuid.UUID) {
	m.UID = uid
}

// SetBusinessID sets the business identifier
func (m *Model) SetBusinessID(id string) {
	m.ID = id
}

// SetVersion sets the version number
func (m *Model) SetVersion(version int) {
	m.Version = version
}

// BeforeCreate sets Version=1 for new business IDs, increments for existing IDs
func (m *Model) BeforeCreate(tx *gorm.DB) error {
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

// IsLatest returns true if this is the latest version (ValidTo is nil)
func (m *Model) IsLatest() bool {
	return m.ValidTo == nil
}

// Close sets ValidTo to the specified time, marking this version as historical
func (m *Model) Close(t time.Time) {
	m.ValidTo = &t
}
