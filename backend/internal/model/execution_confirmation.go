package model

import "time"

// ExecutionConfirmation models 执行确认 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type ExecutionConfirmation struct {
	BaseModel
	Facility    string     `json:"facility" gorm:"size:120;index"`
	Owner       string     `json:"owner" gorm:"size:120;index"`
	Category    string     `json:"category" gorm:"size:80;index"`
	RiskLevel   string     `json:"riskLevel" gorm:"size:32;index"`
	MetricValue float64    `json:"metricValue"`
	MetricUnit  string     `json:"metricUnit" gorm:"size:24"`
	EffectiveAt time.Time  `json:"effectiveAt"`
	Evidence    string     `json:"evidence" gorm:"size:2000"`
	RelatedCode string     `json:"relatedCode" gorm:"size:64;uniqueIndex;not null"`
	ConfirmedBy string     `json:"confirmedBy" gorm:"size:80;index"`
	ConfirmedAt *time.Time `json:"confirmedAt"`
}

func (item *ExecutionConfirmation) GetBase() *BaseModel { return &item.BaseModel }

func (item ExecutionConfirmation) TableName() string { return "execution_confirmations" }

var ExecutionConfirmationInitialStatus = "pending"
