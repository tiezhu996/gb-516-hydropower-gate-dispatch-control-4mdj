package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/dto"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/repository"
)

// prepareApprovedDirective walks a directive through the two-person flow so it
// reaches approved while the shared gate remains free to acquire.
func prepareApprovedDirective(t *testing.T, directives OperationDirectiveService, code string) model.OperationDirective {
	t.Helper()
	ctx := context.Background()
	created, err := directives.Create(ctx, dto.CreateOperationDirective{
		Code: code, Name: "右岸泄洪闸调度", Facility: "右岸坝段", Owner: "运行一组",
		Category: "泄洪", RiskLevel: "high", MetricValue: 35, MetricUnit: "%",
		EffectiveAt: time.Now().UTC().Add(time.Hour), Evidence: "水位与通信核对完成",
		RelatedCode: "GU-TEST", GateState: "closed",
	}, "operator", "req-create-"+code)
	if err != nil {
		t.Fatalf("create directive %s: %v", code, err)
	}
	submitted, err := directives.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "pending", ExpectedVersion: created.Version, Reason: "提交复核",
	}, "operator", model.RoleOperator, "req-submit-"+code)
	if err != nil {
		t.Fatalf("submit directive %s: %v", code, err)
	}
	approved, err := directives.Transition(ctx, submitted.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: submitted.Version, Reason: "独立复核通过",
	}, "reviewer", model.RoleReviewer, "req-approve-"+code)
	if err != nil {
		t.Fatalf("approve directive %s: %v", code, err)
	}
	return approved
}

func TestDuplicateStartOfSameApprovedDirectiveExecutesExactlyOnce(t *testing.T) {
	directives, db := newDirectiveService(t)
	ctx := context.Background()
	approved := prepareApprovedDirective(t, directives, "OD-DUP")

	first, err := directives.Transition(ctx, approved.ID, dto.TransitionRequest{
		Status: "executing", ExpectedVersion: approved.Version, Reason: "现场开工",
	}, "operator", model.RoleOperator, "req-dup-first")
	if err != nil {
		t.Fatalf("first start should succeed: %v", err)
	}

	// A back-to-back duplicate of the same start request must never be reported
	// as occupation by another directive, nor create a second right. Sequential
	// replay is rejected by the executing->executing state machine; the genuine
	// race (both requests reading approved) is mapped to a version conflict in
	// the transaction. Either way exactly one execution and one right survive.
	_, err = directives.Transition(ctx, approved.ID, dto.TransitionRequest{
		Status: "executing", ExpectedVersion: approved.Version, Reason: "双击重复开工",
	}, "operator", model.RoleOperator, "req-dup-second")
	if err == nil {
		t.Fatal("duplicate start must be rejected")
	}
	var occupied *GateOccupiedError
	if errors.As(err, &occupied) {
		t.Fatalf("duplicate start of the same directive must not be blamed on another holder: %#v", occupied)
	}
	stored, _ := directives.Get(ctx, approved.ID)
	if stored.Status != "executing" || stored.Version != first.Version {
		t.Fatalf("directive should execute exactly once: %#v", stored)
	}
	var lockCount int64
	if err := db.Model(&model.GateExecutionLock{}).Count(&lockCount).Error; err != nil || lockCount != 1 {
		t.Fatalf("duplicate start must not create a second right, count=%d err=%v", lockCount, err)
	}
}

func TestGateRightUniqueIndexRejectsConcurrentAcquires(t *testing.T) {
	_, db := newDirectiveService(t)
	ctx := context.Background()
	gateRepo := repository.NewGateUnitRepository(db)
	gate, err := gateRepo.GetByCode(ctx, "GU-TEST")
	if err != nil {
		t.Fatalf("load gate: %v", err)
	}
	locks := repository.NewGateExecutionLockRepository(db)
	first := model.GateExecutionLock{GateID: gate.ID, GateCode: gate.Code, DirectiveID: 101, DirectiveCode: "OD-RACE-A", AcquiredBy: "operator", RequestID: "race-a", AcquiredAt: time.Now().UTC()}
	if err := locks.Acquire(ctx, &first); err != nil {
		t.Fatalf("first acquire should win: %v", err)
	}
	// Even if both requests reach the insert, the gate-level unique index lets
	// exactly one row survive; the loser gets a constraint error to translate.
	second := model.GateExecutionLock{GateID: gate.ID, GateCode: gate.Code, DirectiveID: 202, DirectiveCode: "OD-RACE-B", AcquiredBy: "operator", RequestID: "race-b", AcquiredAt: time.Now().UTC()}
	if err := locks.Acquire(ctx, &second); err == nil {
		t.Fatal("second acquire for the same gate must violate the unique gate index")
	}
	var count int64
	if err := db.Model(&model.GateExecutionLock{}).Where("gate_id = ?", gate.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("exactly one right per gate, count=%d err=%v", count, err)
	}
}

func TestExecutionRightBlocksSecondApprovedDirectiveAndReportsHolder(t *testing.T) {
	directives, db := newDirectiveService(t)
	ctx := context.Background()
	first := prepareApprovedDirective(t, directives, "OD-LOCK-A")
	second := prepareApprovedDirective(t, directives, "OD-LOCK-B")

	executing, err := directives.Transition(ctx, first.ID, dto.TransitionRequest{
		Status: "executing", ExpectedVersion: first.Version, Reason: "现场开工",
	}, "operator", model.RoleOperator, "req-execute-a")
	if err != nil {
		t.Fatalf("first approved directive should acquire execution right: %v", err)
	}
	if executing.Status != "executing" || executing.ExecutionLock == nil || executing.ExecutionLock.DirectiveCode != "OD-LOCK-A" {
		t.Fatalf("first directive should hold the execution right: %#v", executing)
	}

	// The loser keeps its approved state: status, version and gate stay untouched.
	gateRepo := repository.NewGateUnitRepository(db)
	gateBefore, _ := gateRepo.GetByCode(ctx, "GU-TEST")
	_, err = directives.Transition(ctx, second.ID, dto.TransitionRequest{
		Status: "executing", ExpectedVersion: second.Version, Reason: "设备已被占用",
	}, "operator", model.RoleOperator, "req-execute-b")
	var occupied *GateOccupiedError
	if !errors.As(err, &occupied) {
		t.Fatalf("second directive should hit gate occupation, got %v", err)
	}
	if occupied.OccupiedDirectiveCode != "OD-LOCK-A" || occupied.GateCode != "GU-TEST" {
		t.Fatalf("occupation should name holder OD-LOCK-A on GU-TEST: %#v", occupied)
	}
	unchanged, _ := directives.Get(ctx, second.ID)
	if unchanged.Status != "approved" || unchanged.Version != second.Version {
		t.Fatalf("losing directive must keep its original state, got %#v", unchanged)
	}
	gateAfter, _ := gateRepo.GetByCode(ctx, "GU-TEST")
	if gateAfter.Status != gateBefore.Status || gateAfter.Version != gateBefore.Version {
		t.Fatalf("gate must stay untouched for the losing request: before=%#v after=%#v", gateBefore, gateAfter)
	}
	var lockCount int64
	if err := db.Model(&model.GateExecutionLock{}).Count(&lockCount).Error; err != nil || lockCount != 1 {
		t.Fatalf("exactly one execution right must exist, count=%d err=%v", lockCount, err)
	}

	listed, err := directives.List(ctx, dto.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list directives with lock markers: %v", err)
	}
	var holderListed bool
	for _, item := range listed.Items {
		if item.Code == "OD-LOCK-A" && item.ExecutionLock != nil && item.ExecutionLock.DirectiveCode == "OD-LOCK-A" {
			holderListed = true
		}
		if item.Code == "OD-LOCK-B" && item.ExecutionLock != nil {
			t.Fatalf("losing directive must not show an execution right")
		}
	}
	if !holderListed {
		t.Fatal("list response should mark the holding directive with its execution right")
	}
}

func TestExecutionRightReleasedOnAbort(t *testing.T) {
	directives, db := newDirectiveService(t)
	ctx := context.Background()
	first := prepareApprovedDirective(t, directives, "OD-REL-A")
	second := prepareApprovedDirective(t, directives, "OD-REL-B")

	executing, err := directives.Transition(ctx, first.ID, dto.TransitionRequest{
		Status: "executing", ExpectedVersion: first.Version, Reason: "现场开工",
	}, "operator", model.RoleOperator, "req-execute-rel-a")
	if err != nil {
		t.Fatalf("execute first directive: %v", err)
	}
	aborted, err := directives.Transition(ctx, executing.ID, dto.TransitionRequest{
		Status: "aborted", ExpectedVersion: executing.Version, Reason: "工况变化，中止作业",
	}, "operator", model.RoleOperator, "req-abort-rel-a")
	if err != nil {
		t.Fatalf("abort executing directive: %v", err)
	}
	if aborted.Status != "aborted" || aborted.ExecutionLock != nil {
		t.Fatalf("aborted directive must release execution right: %#v", aborted)
	}
	var lockCount int64
	if err := db.Model(&model.GateExecutionLock{}).Count(&lockCount).Error; err != nil || lockCount != 0 {
		t.Fatalf("abort must delete the execution right, count=%d err=%v", lockCount, err)
	}
	gateRepo := repository.NewGateUnitRepository(db)
	gate, _ := gateRepo.GetByCode(ctx, "GU-TEST")
	if gate.Status != "locked" {
		t.Fatalf("aborting an executing directive must physically lock the gate, got %s", gate.Status)
	}

	// Execution right released: the second directive is no longer rejected as
	// "occupied by another directive". It now hits the physical lock rule and
	// keeps its approved state until the gate is unlocked through its own flow.
	_, err = directives.Transition(ctx, second.ID, dto.TransitionRequest{
		Status: "executing", ExpectedVersion: second.Version, Reason: "尝试在闭锁闸门开工",
	}, "operator", model.RoleOperator, "req-execute-rel-b")
	var occupied *GateOccupiedError
	if errors.As(err, &occupied) {
		t.Fatalf("released right must not report occupation by the aborted directive: %#v", occupied)
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("locked gate should reject execution via the physical-lock rule, got %v", err)
	}
	unchanged, _ := directives.Get(ctx, second.ID)
	if unchanged.Status != "approved" || unchanged.Version != second.Version {
		t.Fatalf("second directive must remain approved against a locked gate, got %#v", unchanged)
	}
}

func TestExecutionRightReleasedAfterFailedConfirmation(t *testing.T) {
	confirmations, directives, gates, _, db := newExecutionWorkflow(t)
	executing := prepareExecutingDirective(t, directives)
	var beforeCount int64
	if err := db.Model(&model.GateExecutionLock{}).Where("directive_id = ?", executing.ID).Count(&beforeCount).Error; err != nil || beforeCount != 1 {
		t.Fatalf("executing directive should own a lock, count=%d err=%v", beforeCount, err)
	}

	pending := createPendingConfirmation(t, confirmations)
	failed, err := confirmations.Transition(context.Background(), pending.ID, dto.TransitionRequest{
		Status: "failed", ExpectedVersion: pending.Version, Reason: "开度反馈异常，回执失败",
	}, "operator", "req-confirm-failed")
	if err != nil {
		t.Fatalf("failed confirmation: %v", err)
	}
	if failed.Status != "failed" {
		t.Fatalf("confirmation should be failed, got %s", failed.Status)
	}
	aborted, _ := directives.Get(context.Background(), executing.ID)
	if aborted.Status != "aborted" || aborted.ExecutionLock != nil {
		t.Fatalf("failed receipt must abort directive and release right: %#v", aborted)
	}
	var lockCount int64
	if err := db.Model(&model.GateExecutionLock{}).Count(&lockCount).Error; err != nil || lockCount != 0 {
		t.Fatalf("failed receipt must release the execution right, count=%d err=%v", lockCount, err)
	}
	gate, _ := gates.GetByCode(context.Background(), "GU-FLOW")
	if gate.Status != "locked" {
		t.Fatalf("failed receipt must lock the gate, got %s", gate.Status)
	}
}

func TestExecutionRightReleasedAfterCompletedConfirmation(t *testing.T) {
	confirmations, directives, _, _, db := newExecutionWorkflow(t)
	executing := prepareExecutingDirective(t, directives)
	pending := createPendingConfirmation(t, confirmations)
	if _, err := confirmations.Transition(context.Background(), pending.ID, dto.TransitionRequest{
		Status: "confirmed", ExpectedVersion: pending.Version, Reason: "现场反馈与目标一致",
	}, "operator", "req-confirm-ok"); err != nil {
		t.Fatalf("confirm execution: %v", err)
	}
	completed, _ := directives.Get(context.Background(), executing.ID)
	if completed.Status != "completed" || completed.ExecutionLock != nil {
		t.Fatalf("completed directive must no longer hold the execution right: %#v", completed)
	}
	var lockCount int64
	if err := db.Model(&model.GateExecutionLock{}).Count(&lockCount).Error; err != nil || lockCount != 0 {
		t.Fatalf("completion must release the execution right, count=%d err=%v", lockCount, err)
	}
}

func TestGateAcceptsNextApprovedDirectiveAfterCompletionRelease(t *testing.T) {
	confirmations, directives, gates, _, db := newExecutionWorkflow(t)
	ctx := context.Background()
	prepareExecutingDirective(t, directives)
	pending := createPendingConfirmation(t, confirmations)
	if _, err := confirmations.Transition(ctx, pending.ID, dto.TransitionRequest{
		Status: "confirmed", ExpectedVersion: pending.Version, Reason: "第一批作业完成",
	}, "operator", "req-confirm-finish"); err != nil {
		t.Fatalf("confirm execution: %v", err)
	}
	gate, _ := gates.GetByCode(ctx, "GU-FLOW")
	if gate.ExecutionLock != nil {
		t.Fatalf("gate must report a free execution right after completion: %#v", gate.ExecutionLock)
	}

	created, err := directives.Create(ctx, dto.CreateOperationDirective{
		Code: "OD-FLOW-2", Name: "再次开启泄洪闸", Facility: "主坝", Owner: "运行一组",
		Category: "泄洪", RiskLevel: "high", MetricValue: 35, MetricUnit: "%",
		EffectiveAt: time.Now().UTC().Add(time.Hour), Evidence: "下一个许可窗口",
		RelatedCode: "GU-FLOW", GateState: "open",
	}, "operator", "req-create-2")
	if err != nil {
		t.Fatalf("create follow-up directive: %v", err)
	}
	submitted, err := directives.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "pending", ExpectedVersion: created.Version, Reason: "提交复核",
	}, "operator", model.RoleOperator, "req-submit-2")
	if err != nil {
		t.Fatalf("submit follow-up: %v", err)
	}
	approved, err := directives.Transition(ctx, submitted.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: submitted.Version, Reason: "独立复核通过",
	}, "reviewer", model.RoleReviewer, "req-approve-2")
	if err != nil {
		t.Fatalf("approve follow-up: %v", err)
	}
	next, err := directives.Transition(ctx, approved.ID, dto.TransitionRequest{
		Status: "executing", ExpectedVersion: approved.Version, Reason: "前一条已完成，闸门空闲，开工",
	}, "operator", model.RoleOperator, "req-execute-2")
	if err != nil {
		t.Fatalf("freed gate should accept the next approved directive: %v", err)
	}
	if next.ExecutionLock == nil || next.ExecutionLock.DirectiveCode != "OD-FLOW-2" {
		t.Fatalf("follow-up directive should hold the execution right: %#v", next)
	}
	var count int64
	if err := db.Model(&model.GateExecutionLock{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("only the follow-up lock should remain, count=%d err=%v", count, err)
	}
}
