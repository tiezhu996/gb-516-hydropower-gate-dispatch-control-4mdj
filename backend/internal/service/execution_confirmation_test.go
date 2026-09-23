package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/config"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/dto"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newExecutionWorkflow(t *testing.T) (ExecutionConfirmationService, OperationDirectiveService, repository.GateUnitRepository, repository.OperationDirectiveRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.GateUnit{}, &model.OperationDirective{}, &model.DirectiveApproval{}, &model.ExecutionConfirmation{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
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

func prepareExecutingDirective(t *testing.T, directives OperationDirectiveService) model.OperationDirective {
	t.Helper()
	ctx := context.Background()
	created, err := directives.Create(ctx, dto.CreateOperationDirective{
		Code: "OD-FLOW", Name: "开启泄洪闸", Facility: "主坝", Owner: "运行一组",
		Category: "泄洪", RiskLevel: "high", MetricValue: 35, MetricUnit: "%",
		EffectiveAt: time.Now().UTC().Add(time.Hour), Evidence: "水位与通信核对完成",
		RelatedCode: "GU-FLOW", GateState: "open",
	}, "operator", "req-create")
	if err != nil {
		t.Fatalf("create directive: %v", err)
	}
	submitted, err := directives.Transition(ctx, created.ID, dto.TransitionRequest{Status: "pending", ExpectedVersion: created.Version, Reason: "提交复核"}, "operator", model.RoleOperator, "req-submit")
	if err != nil {
		t.Fatalf("submit directive: %v", err)
	}
	approved, err := directives.Transition(ctx, submitted.ID, dto.TransitionRequest{Status: "approved", ExpectedVersion: submitted.Version, Reason: "独立复核通过"}, "reviewer", model.RoleReviewer, "req-approve")
	if err != nil {
		t.Fatalf("approve directive: %v", err)
	}
	executing, err := directives.Transition(ctx, approved.ID, dto.TransitionRequest{Status: "executing", ExpectedVersion: approved.Version, Reason: "现场开始执行"}, "operator", model.RoleOperator, "req-execute")
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
