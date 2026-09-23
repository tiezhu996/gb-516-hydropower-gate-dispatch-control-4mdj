package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/config"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/database"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/dto"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newExecutionWorkflow(t *testing.T) (ExecutionConfirmationService, OperationDirectiveService, repository.GateUnitRepository, repository.OperationDirectiveRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.GateUnit{}, &model.OperationDirective{}, &model.DirectiveApproval{}, &model.ExecutionConfirmation{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	if err := database.EnsureDirectiveExecutionLock(db); err != nil {
		t.Fatalf("install gate execution lock: %v", err)
	}
	gateRepo := repository.NewGateUnitRepository(db)
	directiveRepo := repository.NewOperationDirectiveRepository(db)
	confirmationRepo := repository.NewExecutionConfirmationRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	directives := NewOperationDirectiveService(directiveRepo, gateRepo, security)
	confirmations := NewExecutionConfirmationService(confirmationRepo, directiveRepo, gateRepo, security)
	gate := model.GateUnit{BaseModel: model.BaseModel{Code: "GU-FLOW", Name: "泄洪闸", Status: "closed", Version: 1}, Facility: "主坝", Owner: "运行一组"}
	if err := gateRepo.Create(context.Background(), &gate); err != nil {
		t.Fatalf("create gate: %v", err)
	}
	return confirmations, directives, gateRepo, directiveRepo, db
}

func approveDirectiveOnGate(t *testing.T, directives OperationDirectiveService, code, gateCode string) model.OperationDirective {
	t.Helper()
	ctx := context.Background()
	created, err := directives.Create(ctx, dto.CreateOperationDirective{
		Code: code, Name: "开启泄洪闸", Facility: "主坝", Owner: "运行一组",
		Category: "泄洪", RiskLevel: "high", MetricValue: 35, MetricUnit: "%",
		EffectiveAt: time.Now().UTC().Add(time.Hour), Evidence: "水位与通信核对完成",
		RelatedCode: gateCode, GateState: "open",
	}, "operator", "req-create-"+code)
	if err != nil {
		t.Fatalf("create directive: %v", err)
	}
	submitted, err := directives.Transition(ctx, created.ID, dto.TransitionRequest{Status: "pending", ExpectedVersion: created.Version, Reason: "提交复核"}, "operator", model.RoleOperator, "req-submit-"+code)
	if err != nil {
		t.Fatalf("submit directive: %v", err)
	}
	approved, err := directives.Transition(ctx, submitted.ID, dto.TransitionRequest{Status: "approved", ExpectedVersion: submitted.Version, Reason: "独立复核通过"}, "reviewer", model.RoleReviewer, "req-approve-"+code)
	if err != nil {
		t.Fatalf("approve directive: %v", err)
	}
	return approved
}

func prepareExecutingDirective(t *testing.T, directives OperationDirectiveService) model.OperationDirective {
	t.Helper()
	approved := approveDirectiveOnGate(t, directives, "OD-FLOW", "GU-FLOW")
	executing, err := directives.Transition(context.Background(), approved.ID, dto.TransitionRequest{Status: "executing", ExpectedVersion: approved.Version, Reason: "现场开始执行"}, "operator", model.RoleOperator, "req-execute")
	if err != nil {
		t.Fatalf("execute directive: %v", err)
	}
	return executing
}

func createPendingConfirmation(t *testing.T, confirmations ExecutionConfirmationService) model.ExecutionConfirmation {
	t.Helper()
	item, err := confirmations.Create(context.Background(), dto.CreateExecutionConfirmation{
		Code: "EC-FLOW", Name: "现场执行回执", Facility: "主坝", Owner: "现场操作员",
		Category: "执行", RiskLevel: "high", MetricValue: 35, MetricUnit: "%",
		EffectiveAt: time.Now().UTC(), Evidence: "开度反馈与视频记录一致", RelatedCode: "OD-FLOW",
	}, "operator", "req-confirm-create")
	if err != nil {
		t.Fatalf("create confirmation: %v", err)
	}
	return item
}

func TestExecutionConfirmationCompletesDirectiveAndGateAtomically(t *testing.T) {
	confirmations, directives, gates, _, db := newExecutionWorkflow(t)
	executing := prepareExecutingDirective(t, directives)
	gate, _ := gates.GetByCode(context.Background(), "GU-FLOW")
	if gate.Status != "moving" {
		t.Fatalf("gate should enter moving when directive executes, got %s", gate.Status)
	}
	pending := createPendingConfirmation(t, confirmations)
	confirmed, err := confirmations.Transition(context.Background(), pending.ID, dto.TransitionRequest{
		Status: "confirmed", ExpectedVersion: pending.Version, Reason: "目标开度和现场反馈一致",
	}, "operator", "req-confirm")
	if err != nil {
		t.Fatalf("confirm execution: %v", err)
	}
	updatedDirective, _ := directives.Get(context.Background(), executing.ID)
	updatedGate, _ := gates.GetByCode(context.Background(), "GU-FLOW")
	if confirmed.Status != "confirmed" || updatedDirective.Status != "completed" || updatedGate.Status != "open" {
		t.Fatalf("workflow not completed atomically: confirmation=%s directive=%s gate=%s", confirmed.Status, updatedDirective.Status, updatedGate.Status)
	}
	var linkedAudits int64
	if err := db.Model(&model.AuditLog{}).Where("request_id = ?", "req-confirm").Count(&linkedAudits).Error; err != nil || linkedAudits != 3 {
		t.Fatalf("expected three linked audit events, count=%d err=%v", linkedAudits, err)
	}
}

func TestExecutionConfirmationRollsBackAllStateWhenAuditFails(t *testing.T) {
	confirmations, directives, gates, _, db := newExecutionWorkflow(t)
	executing := prepareExecutingDirective(t, directives)
	pending := createPendingConfirmation(t, confirmations)
	if err := db.Migrator().DropTable(&model.AuditLog{}); err != nil {
		t.Fatalf("drop audit table: %v", err)
	}
	_, err := confirmations.Transition(context.Background(), pending.ID, dto.TransitionRequest{
		Status: "confirmed", ExpectedVersion: pending.Version, Reason: "审计失败必须整体回滚",
	}, "operator", "req-rollback")
	if err == nil {
		t.Fatal("transition should fail without audit persistence")
	}
	storedConfirmation, _ := confirmations.Get(context.Background(), pending.ID)
	storedDirective, _ := directives.Get(context.Background(), executing.ID)
	storedGate, _ := gates.GetByCode(context.Background(), "GU-FLOW")
	if storedConfirmation.Status != "pending" || storedDirective.Status != "executing" || storedGate.Status != "moving" {
		t.Fatalf("partial state persisted: confirmation=%s directive=%s gate=%s", storedConfirmation.Status, storedDirective.Status, storedGate.Status)
	}
}

func TestCompletedDirectiveReleasesGateExecutionLock(t *testing.T) {
	confirmations, directives, _, _, _ := newExecutionWorkflow(t)
	ctx := context.Background()
	prepareExecutingDirective(t, directives)
	pending := createPendingConfirmation(t, confirmations)
	if _, err := confirmations.Transition(ctx, pending.ID, dto.TransitionRequest{
		Status: "confirmed", ExpectedVersion: pending.Version, Reason: "目标开度和现场反馈一致",
	}, "operator", "req-confirm"); err != nil {
		t.Fatalf("confirm execution: %v", err)
	}
	successor := approveDirectiveOnGate(t, directives, "OD-SUCCESSOR", "GU-FLOW")
	started, err := directives.Transition(ctx, successor.ID, dto.TransitionRequest{
		Status: "executing", ExpectedVersion: successor.Version, Reason: "前序指令已完成，闸门执行权已释放",
	}, "operator", model.RoleOperator, "req-execute-successor")
	if err != nil {
		t.Fatalf("completed directive must release the gate execution lock: %v", err)
	}
	if started.Status != "executing" || started.GateOccupier != "OD-SUCCESSOR" {
		t.Fatalf("successor should hold the gate, got %s occupier=%q", started.Status, started.GateOccupier)
	}
}

func TestFailedConfirmationReleasesGateExecutionLock(t *testing.T) {
	confirmations, directives, _, _, db := newExecutionWorkflow(t)
	ctx := context.Background()
	executing := prepareExecutingDirective(t, directives)

	challenger := approveDirectiveOnGate(t, directives, "OD-CHALLENGER", "GU-FLOW")
	_, err := directives.Transition(ctx, challenger.ID, dto.TransitionRequest{
		Status: "executing", ExpectedVersion: challenger.Version, Reason: "闸门被占用时必须拒绝",
	}, "operator", model.RoleOperator, "req-challenger-early")
	if !errors.Is(err, ErrGateOccupied) {
		t.Fatalf("challenger should be rejected while the gate is occupied, got %v", err)
	}

	pending := createPendingConfirmation(t, confirmations)
	if _, err := confirmations.Transition(ctx, pending.ID, dto.TransitionRequest{
		Status: "failed", ExpectedVersion: pending.Version, Reason: "现场执行失败，回执失败并释放执行权",
	}, "operator", "req-fail"); err != nil {
		t.Fatalf("fail execution confirmation: %v", err)
	}
	aborted, err := directives.Get(ctx, executing.ID)
	if err != nil {
		t.Fatalf("reload executing directive: %v", err)
	}
	if aborted.Status != "aborted" || aborted.GateOccupier != "" {
		t.Fatalf("failed receipt must abort the directive and free the gate, got %s occupier=%q", aborted.Status, aborted.GateOccupier)
	}
	// The failed receipt locks the gate; the gate workflow unlocks it before the next start.
	if err := db.Model(&model.GateUnit{}).Where("code = ?", "GU-FLOW").Update("status", "closed").Error; err != nil {
		t.Fatalf("unlock gate: %v", err)
	}
	started, err := directives.Transition(ctx, challenger.ID, dto.TransitionRequest{
		Status: "executing", ExpectedVersion: challenger.Version, Reason: "失败回执已释放闸门执行权",
	}, "operator", model.RoleOperator, "req-challenger-start")
	if err != nil {
		t.Fatalf("failed confirmation must release the gate execution lock: %v", err)
	}
	if started.Status != "executing" || started.GateOccupier != "OD-CHALLENGER" {
		t.Fatalf("challenger should hold the gate after the failed receipt, got %s occupier=%q", started.Status, started.GateOccupier)
	}
}
