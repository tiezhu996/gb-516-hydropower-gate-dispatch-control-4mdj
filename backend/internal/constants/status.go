package constants

// Shared status values are mirrored in frontend/src/types/status.ts. Keeping
// the lists explicit makes state-machine drift visible during code review.

type GateState string

const (
	GateStateOpen   GateState = "open"
	GateStateClosed GateState = "closed"
	GateStateMoving GateState = "moving"
	GateStateLocked GateState = "locked"
)

var AllGateState = []string{"open", "closed", "moving", "locked"}

type DirectiveState string

const (
	DirectiveStateDraft     DirectiveState = "draft"
	DirectiveStatePending   DirectiveState = "pending"
	DirectiveStateApproved  DirectiveState = "approved"
	DirectiveStateExecuting DirectiveState = "executing"
	DirectiveStateCompleted DirectiveState = "completed"
	DirectiveStateAborted   DirectiveState = "aborted"
)

var AllDirectiveState = []string{"draft", "pending", "approved", "executing", "completed", "aborted"}

var ReservoirTransitions = map[string]map[string]bool{
	"normal":     {"warning": true, "critical": true},
	"warning":    {"critical": true, "restricted": true, "normal": true},
	"critical":   {"restricted": true, "warning": true},
	"restricted": {"critical": true},
}

var GateUnitTransitions = map[string]map[string]bool{
	"open":   {"moving": true, "locked": true},
	"closed": {"moving": true, "locked": true},
	"moving": {"open": true, "closed": true, "locked": true},
	"locked": {"closed": true},
}

var OperationDirectiveTransitions = map[string]map[string]bool{
	"draft":     {"pending": true},
	"pending":   {"approved": true, "aborted": true},
	"approved":  {"executing": true, "aborted": true},
	"executing": {"completed": true, "aborted": true},
	"completed": {},
	"aborted":   {},
}

var ExecutionConfirmationTransitions = map[string]map[string]bool{
	"pending":   {"confirmed": true, "failed": true},
	"confirmed": {},
	"failed":    {"cancelled": true},
	"cancelled": {},
}

func CanTransition(graph map[string]map[string]bool, from, to string) bool {
	targets, exists := graph[from]
	return exists && targets[to]
}
