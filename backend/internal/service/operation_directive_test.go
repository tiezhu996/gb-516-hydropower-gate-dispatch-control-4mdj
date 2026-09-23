package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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

func newDirectiveService(t *testing.T) (OperationDirectiveService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.GateUnit{}, &model.OperationDirective{}, &model.DirectiveApproval{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	if err := database.EnsureDirectiveExecutionLock(db); err != nil {
		t.Fatalf("install gate execution lock: %v", err)
	}
	gate := model.GateUnit{BaseModel: model.BaseModel{Code: "GU-TEST", Name: "右岸泄洪闸", Status: "closed", Version: 1}, Facility: "右岸坝段", Owner: "运行一组"}
	if err := db.Create(&gate).Error; err != nil {
		t.Fatalf("create test gate: %v", err)
	}
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	return NewOperationDirectiveService(repository.NewOperationDirectiveRepository(db), repository.NewGateUnitRepository(db), security), db
}

func directiveInput(code string) dto.CreateOperationDirective {
	return dto.CreateOperationDirective{
		Code: code, Name: "右岸泄洪闸调度", Description: "测试双人确认和状态证据",
		Facility: "右岸坝段", Owner: "运行一组", Category: "泄洪调度", RiskLevel: "high",
		MetricValue: 35, MetricUnit: "%", EffectiveAt: time.Now().UTC().Add(time.Hour),
		Evidence: "水位 168.2m，处于许可窗口", RelatedCode: "GU-TEST", GateState: "closed",
	}
}

func TestDirectiveRequiresIndependentReviewerAndPreservesEvidence(t *testing.T) {
	service, db := newDirectiveService(t)
	ctx := context.Background()
	created, err := service.Create(ctx, directiveInput("OD-TEST-1"), "operator", "req-create")
	if err != nil {
		t.Fatalf("create directive: %v", err)
	}
	submitted, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "pending", ExpectedVersion: created.Version, Reason: "提交水位窗口和开度计划复核",
	}, "operator", model.RoleOperator, "req-submit")
	if err != nil {
		t.Fatalf("submit directive: %v", err)
	}
	if submitted.SubmittedBy != "operator" || len(submitted.Approvals) != 1 || submitted.Approvals[0].RequestID != "req-submit" {
		t.Fatalf("submission evidence not preserved: %#v", submitted)
	}

	_, err = service.Transition(ctx, submitted.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: submitted.Version, Reason: "操作员不得自行批准",
	}, "operator", model.RoleOperator, "req-self-operator")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("operator approval should be forbidden, got %v", err)
	}
	_, err = service.Transition(ctx, submitted.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: submitted.Version, Reason: "同一账号不得切换角色自批",
	}, "operator", model.RoleReviewer, "req-self-reviewer")
	if !errors.Is(err, ErrTwoPersonRequired) {
		t.Fatalf("same-account approval should fail two-person rule, got %v", err)
	}

	approved, err := service.Transition(ctx, submitted.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: submitted.Version, Reason: "复核闸门目标、水位窗口及证据一致",
	}, "reviewer", model.RoleReviewer, "req-approve")
	if err != nil {
		t.Fatalf("reviewer approve directive: %v", err)
	}
	if approved.ApprovedBy != "reviewer" || approved.SubmittedBy == approved.ApprovedBy || len(approved.Approvals) != 2 {
		t.Fatalf("two-person evidence invalid: %#v", approved)
	}
	if approved.Approvals[1].Stage != "approved" || approved.Approvals[1].RequestID != "req-approve" {
		t.Fatalf("approval trail invalid: %#v", approved.Approvals)
	}
	var transitions int64
	if err := db.Model(&model.AuditLog{}).Where("entity_type = ? AND entity_id = ? AND action = ?", "OperationDirective", approved.ID, "transition").Count(&transitions).Error; err != nil {
		t.Fatalf("count transition audits: %v", err)
	}
	if transitions != 2 {
		t.Fatalf("expected two transition audits, got %d", transitions)
	}

	_, err = service.Update(ctx, approved.ID, dto.UpdateOperationDirective{ExpectedVersion: approved.Version}, "operator", "req-edit")
	if !errors.Is(err, ErrImmutableState) {
		t.Fatalf("submitted directive must be immutable, got %v", err)
	}
}

func TestDirectiveTransitionRollsBackWhenAuditCannotPersist(t *testing.T) {
	service, db := newDirectiveService(t)
	ctx := context.Background()
	created, err := service.Create(ctx, directiveInput("OD-TEST-ROLLBACK"), "operator", "req-create")
	if err != nil {
		t.Fatalf("create directive: %v", err)
	}
	if err := db.Migrator().DropTable(&model.AuditLog{}); err != nil {
		t.Fatalf("drop audit table: %v", err)
	}
	_, err = service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "pending", ExpectedVersion: created.Version, Reason: "应与审计失败一起回滚",
	}, "operator", model.RoleOperator, "req-rollback")
	if err == nil {
		t.Fatal("transition should fail when audit cannot persist")
	}
	stored, getErr := service.Get(ctx, created.ID)
	if getErr != nil {
		t.Fatalf("reload rolled-back directive: %v", getErr)
	}
	if stored.Status != "draft" || stored.Version != created.Version || len(stored.Approvals) != 0 {
		t.Fatalf("transition was not fully rolled back: %#v", stored)
	}
}

func TestDirectiveCreateRollsBackWhenAuditCannotPersist(t *testing.T) {
	service, db := newDirectiveService(t)
	if err := db.Migrator().DropTable(&model.AuditLog{}); err != nil {
		t.Fatalf("drop audit table: %v", err)
	}
	_, err := service.Create(context.Background(), directiveInput("OD-CREATE-ROLLBACK"), "operator", "req-create-rollback")
	if err == nil {
		t.Fatal("create should fail when audit cannot persist")
	}
	var count int64
	if err := db.Model(&model.OperationDirective{}).Where("code = ?", "OD-CREATE-ROLLBACK").Count(&count).Error; err != nil {
		t.Fatalf("count directives: %v", err)
	}
	if count != 0 {
		t.Fatalf("directive persisted without audit: count=%d", count)
	}
}

// approveDirective walks a draft through submission and independent approval
// so execution-lock scenarios start from a valid approved record.
func approveDirective(t *testing.T, service OperationDirectiveService, code string) model.OperationDirective {
	t.Helper()
	ctx := context.Background()
	input := directiveInput(code)
	input.GateState = "open"
	created, err := service.Create(ctx, input, "operator", "req-create-"+code)
	if err != nil {
		t.Fatalf("create directive %s: %v", code, err)
	}
	submitted, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "pending", ExpectedVersion: created.Version, Reason: "提交水位窗口和开度计划复核",
	}, "operator", model.RoleOperator, "req-submit-"+code)
	if err != nil {
		t.Fatalf("submit directive %s: %v", code, err)
	}
	approved, err := service.Transition(ctx, submitted.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: submitted.Version, Reason: "复核闸门目标、水位窗口及证据一致",
	}, "reviewer", model.RoleReviewer, "req-approve-"+code)
	if err != nil {
		t.Fatalf("approve directive %s: %v", code, err)
	}
	return approved
}

func executeDirective(service OperationDirectiveService, item model.OperationDirective, requestID string) (model.OperationDirective, error) {
	return service.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "executing", ExpectedVersion: item.Version, Reason: "双人许可完成，现场开始执行",
	}, "operator", model.RoleOperator, requestID)
}

func TestDirectiveExecutionLockBlocksSecondDirective(t *testing.T) {
	service, db := newDirectiveService(t)
	ctx := context.Background()
	first := approveDirective(t, service, "OD-LOCK-1")
	second := approveDirective(t, service, "OD-LOCK-2")

	executing, err := executeDirective(service, first, "req-execute-1")
	if err != nil {
		t.Fatalf("first directive should acquire the idle gate: %v", err)
	}
	if executing.GateOccupier != "OD-LOCK-1" {
		t.Fatalf("executing directive should report itself as gate occupier, got %q", executing.GateOccupier)
	}

	_, err = executeDirective(service, second, "req-execute-2")
	if !errors.Is(err, ErrGateOccupied) {
		t.Fatalf("second directive should be rejected with ErrGateOccupied, got %v", err)
	}
	if !strings.Contains(err.Error(), "OD-LOCK-1") {
		t.Fatalf("occupied error should name the occupant directive, got %v", err)
	}
	stored, getErr := service.Get(ctx, second.ID)
	if getErr != nil {
		t.Fatalf("reload rejected directive: %v", getErr)
	}
	if stored.Status != "approved" || stored.Version != second.Version {
		t.Fatalf("rejected directive must keep its approved state, got %s v%d", stored.Status, stored.Version)
	}
	if stored.GateOccupier != "OD-LOCK-1" {
		t.Fatalf("rejected directive should see the current occupier, got %q", stored.GateOccupier)
	}
	var gate model.GateUnit
	if err := db.First(&gate, "code = ?", "GU-TEST").Error; err != nil {
		t.Fatalf("reload gate: %v", err)
	}
	if gate.Status != "moving" {
		t.Fatalf("gate must stay with the occupying directive, got %s", gate.Status)
	}
	page, err := service.List(ctx, dto.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list directives: %v", err)
	}
	for _, item := range page.Items {
		if item.GateOccupier != "OD-LOCK-1" {
			t.Fatalf("directive %s should list occupier OD-LOCK-1, got %q", item.Code, item.GateOccupier)
		}
	}
}

func TestDirectiveExecutionLockAllowsOnlyOneConcurrentWinner(t *testing.T) {
	service, db := newDirectiveService(t)
	first := approveDirective(t, service, "OD-RACE-1")
	second := approveDirective(t, service, "OD-RACE-2")

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, item := range []model.OperationDirective{first, second} {
		wg.Add(1)
		go func(directive model.OperationDirective) {
			defer wg.Done()
			_, err := executeDirective(service, directive, "req-race-"+directive.Code)
			results <- err
		}(item)
	}
	wg.Wait()
	close(results)
	var successes, occupied int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrGateOccupied):
			occupied++
		default:
			t.Fatalf("unexpected race outcome: %v", err)
		}
	}
	if successes != 1 || occupied != 1 {
		t.Fatalf("exactly one starter may win the gate, successes=%d occupied=%d", successes, occupied)
	}
	var executingCount int64
	if err := db.Model(&model.OperationDirective{}).Where("status = ?", "executing").Count(&executingCount).Error; err != nil {
		t.Fatalf("count executing directives: %v", err)
	}
	if executingCount != 1 {
		t.Fatalf("gate must hold exactly one executing directive, got %d", executingCount)
	}
}

func TestDirectiveExecutionLockReleasesAfterAbort(t *testing.T) {
	service, db := newDirectiveService(t)
	ctx := context.Background()
	first := approveDirective(t, service, "OD-RELEASE-1")
	second := approveDirective(t, service, "OD-RELEASE-2")

	executing, err := executeDirective(service, first, "req-execute-release-1")
	if err != nil {
		t.Fatalf("first directive should acquire the idle gate: %v", err)
	}
	if _, err := service.Transition(ctx, executing.ID, dto.TransitionRequest{
		Status: "aborted", ExpectedVersion: executing.Version, Reason: "现场中止，释放闸门执行权",
	}, "operator", model.RoleOperator, "req-abort-release"); err != nil {
		t.Fatalf("abort executing directive: %v", err)
	}
	// Aborting locks the gate; the gate workflow unlocks it before the next start.
	if err := db.Model(&model.GateUnit{}).Where("code = ?", "GU-TEST").Update("status", "closed").Error; err != nil {
		t.Fatalf("unlock gate: %v", err)
	}
	started, err := executeDirective(service, second, "req-execute-release-2")
	if err != nil {
		t.Fatalf("gate execution lock must be released after abort: %v", err)
	}
	if started.Status != "executing" || started.GateOccupier != "OD-RELEASE-2" {
		t.Fatalf("second directive should now hold the gate, got %s occupier=%q", started.Status, started.GateOccupier)
	}
}
