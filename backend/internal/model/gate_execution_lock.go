package model

import "time"

// GateExecutionLock is the single execution right attached to a 闸门 while an
// approved 操作指令 is running. Exactly one row may exist per gate and per
// directive: the two unique indexes make concurrent "start execution" requests
// safe even when they arrive back-to-back. Releasing the right deletes the row.
//
// This is the dispatch-level execution right and is distinct from the gate's
// physical status (GateUnit.Status == "locked"). Aborting a directive or a
// failed receipt both release the right AND physically lock the gate; the
// physical lock is cleared through the gate's own state machine afterwards.
type GateExecutionLock struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	GateID        uint      `json:"gateId" gorm:"not null;uniqueIndex:idx_execution_lock_gate"`
	GateCode      string    `json:"gateCode" gorm:"size:64;not null;index"`
	DirectiveID   uint      `json:"directiveId" gorm:"not null;uniqueIndex:idx_execution_lock_directive"`
	DirectiveCode string    `json:"directiveCode" gorm:"size:64;not null;index"`
	AcquiredBy    string    `json:"acquiredBy" gorm:"size:80;not null"`
	RequestID     string    `json:"requestId" gorm:"size:64;not null"`
	AcquiredAt    time.Time `json:"acquiredAt" gorm:"not null;index"`
}

func (GateExecutionLock) TableName() string { return "gate_execution_locks" }
