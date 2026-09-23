package constants

import "testing"

func TestReservoirTransitionGraph(t *testing.T) {
	if !CanTransition(ReservoirTransitions, "normal", "warning") {
		t.Fatalf("expected normal -> warning transition to be allowed")
	}
	if CanTransition(ReservoirTransitions, "normal", "unknown") {
		t.Fatal("unknown status must never be accepted")
	}
}

func TestSafetyCriticalTransitionsCannotSkipRequiredStages(t *testing.T) {
	if CanTransition(OperationDirectiveTransitions, "draft", "approved") {
		t.Fatal("directive must not skip independent review")
	}
	if CanTransition(OperationDirectiveTransitions, "pending", "executing") {
		t.Fatal("pending directive must not execute before approval")
	}
	if CanTransition(GateUnitTransitions, "closed", "open") {
		t.Fatal("gate must pass through moving state")
	}
	if CanTransition(ExecutionConfirmationTransitions, "confirmed", "failed") {
		t.Fatal("confirmed execution receipt must be terminal")
	}
}
