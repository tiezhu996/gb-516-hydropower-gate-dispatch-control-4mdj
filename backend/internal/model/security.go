package model

import "time"

type User struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"size:80;uniqueIndex;not null"`
	DisplayName  string    `json:"displayName" gorm:"size:120;not null"`
	PasswordHash string    `json:"-" gorm:"size:120;not null"`
	Role         string    `json:"role" gorm:"size:32;index;not null"`
	Active       bool      `json:"active" gorm:"not null;default:true"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type AuditLog struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	RequestID   string    `json:"requestId" gorm:"size:64;index"`
	Actor       string    `json:"actor" gorm:"size:80;index"`
	Action      string    `json:"action" gorm:"size:80;index"`
	EntityType  string    `json:"entityType" gorm:"size:80;index"`
	EntityID    uint      `json:"entityId" gorm:"index"`
	BeforeState string    `json:"beforeState" gorm:"size:40"`
	AfterState  string    `json:"afterState" gorm:"size:40"`
	Detail      string    `json:"detail" gorm:"size:2000"`
	CreatedAt   time.Time `json:"createdAt" gorm:"index"`
}

type AuditActionCount struct {
	Action string `json:"action" gorm:"column:action"`
	Count  int64  `json:"count" gorm:"column:count"`
}

type AuditEntityCount struct {
	EntityType string `json:"entityType" gorm:"column:entity_type"`
	Count      int64  `json:"count" gorm:"column:count"`
}

type AuditSummary struct {
	Since        time.Time          `json:"since"`
	Total        int64              `json:"total"`
	Transitions  int64              `json:"transitions"`
	UniqueActors int64              `json:"uniqueActors"`
	Actions      []AuditActionCount `json:"actions"`
	EntityTypes  []AuditEntityCount `json:"entityTypes"`
}

const (
	RoleViewer   = "viewer"
	RoleOperator = "operator"
	RoleReviewer = "reviewer"
	RoleAdmin    = "admin"
)
