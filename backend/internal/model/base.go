package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel provides stable identifiers and optimistic locking for every
// aggregate. Code is the human-facing identifier and remains immutable.
type BaseModel struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Code        string         `json:"code" gorm:"size:64;uniqueIndex;not null"`
	Name        string         `json:"name" gorm:"size:160;not null"`
	Status      string         `json:"status" gorm:"size:40;index;not null"`
	Version     uint           `json:"version" gorm:"not null;default:1"`
	Description string         `json:"description" gorm:"size:1000"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (b *BaseModel) BeforeCreate(_ *gorm.DB) error {
	if b.Version == 0 {
		b.Version = 1
	}
	return nil
}

type DomainRecord interface {
	GetBase() *BaseModel
}
