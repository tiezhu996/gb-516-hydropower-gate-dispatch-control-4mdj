package model

import "time"

// OperationDirective models 操作指令 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type OperationDirective struct {
	BaseModel
	Facility    string              `json:"facility" gorm:"size:120;index"`
	Owner       string              `json:"owner" gorm:"size:120;index"`
	Category    string              `json:"category" gorm:"size:80;index"`
	RiskLevel   string              `json:"riskLevel" gorm:"size:32;index"`
	MetricValue float64             `json:"metricValue"`
	MetricUnit  string              `json:"metricUnit" gorm:"size:24"`
	EffectiveAt time.Time           `json:"effectiveAt"`
	Evidence    string              `json:"evidence" gorm:"size:2000"`
	RelatedCode string              `json:"relatedCode" gorm:"size:64;index"`
	GateState   string              `json:"gateState" gorm:"size:32;not null;default:closed"`
	SubmittedBy string              `json:"submittedBy" gorm:"size:80;index"`
	SubmittedAt *time.Time          `json:"submittedAt"`
	ApprovedBy  string              `json:"approvedBy" gorm:"size:80;index"`
	ApprovedAt  *time.Time          `json:"approvedAt"`
	// GateOccupier is the code of the directive currently holding the
	// execution right on the linked gate. Derived on read, never persisted.
	GateOccupier string              `json:"gateOccupier" gorm:"-"`
	Approvals    []DirectiveApproval `json:"approvals" gorm:"foreignKey:DirectiveID;constraint:OnDelete:CASCADE"`
}

func (item *OperationDirective) GetBase() *BaseModel { return &item.BaseModel }

func (item OperationDirective) TableName() string { return "operation_directives" }

var OperationDirectiveInitialStatus = "draft"

// DirectiveApproval is append-only evidence for the two-person dispatch
// decision. No update or delete operation is exposed for this table.
type DirectiveApproval struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	DirectiveID uint      `json:"directiveId" gorm:"not null;index;uniqueIndex:idx_directive_stage"`
	Stage       string    `json:"stage" gorm:"size:32;not null;uniqueIndex:idx_directive_stage"`
	Actor       string    `json:"actor" gorm:"size:80;not null;index"`
	Role        string    `json:"role" gorm:"size:32;not null"`
	RequestID   string    `json:"requestId" gorm:"size:64;not null;index"`
	Reason      string    `json:"reason" gorm:"size:500;not null"`
	FromState   string    `json:"fromState" gorm:"size:40;not null"`
	ToState     string    `json:"toState" gorm:"size:40;not null"`
	CreatedAt   time.Time `json:"createdAt" gorm:"index"`
}
