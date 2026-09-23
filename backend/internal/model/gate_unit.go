package model

import "time"

// GateUnit models 闸门 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type GateUnit struct {
	BaseModel
	Facility    string    `json:"facility" gorm:"size:120;index"`
	Owner       string    `json:"owner" gorm:"size:120;index"`
	Category    string    `json:"category" gorm:"size:80;index"`
	RiskLevel   string    `json:"riskLevel" gorm:"size:32;index"`
	MetricValue float64   `json:"metricValue"`
	MetricUnit  string    `json:"metricUnit" gorm:"size:24"`
	EffectiveAt time.Time `json:"effectiveAt"`
	Evidence    string    `json:"evidence" gorm:"size:2000"`
	RelatedCode string    `json:"relatedCode" gorm:"size:64;index"`
	// ExecutionLock is populated by the service with the directive that
	// currently holds this gate's execution right; it is not a persisted column.
	ExecutionLock *GateExecutionLock `json:"executionLock,omitempty" gorm:"-"`
}

func (item *GateUnit) GetBase() *BaseModel { return &item.BaseModel }

func (item GateUnit) TableName() string { return "gate_units" }

var GateUnitInitialStatus = "open"
