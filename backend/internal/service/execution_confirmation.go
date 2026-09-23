package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/constants"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/dto"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/repository"
)

type ExecutionConfirmationService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.ExecutionConfirmation], error)
	Get(context.Context, uint) (model.ExecutionConfirmation, error)
	Create(context.Context, dto.CreateExecutionConfirmation, string, string) (model.ExecutionConfirmation, error)
	Update(context.Context, uint, dto.UpdateExecutionConfirmation, string, string) (model.ExecutionConfirmation, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.ExecutionConfirmation, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type executionConfirmationService struct {
	repository repository.ExecutionConfirmationRepository
	directives repository.OperationDirectiveRepository
	gates      repository.GateUnitRepository
	locks      repository.GateExecutionLockRepository
	security   SecurityService
}

func NewExecutionConfirmationService(repo repository.ExecutionConfirmationRepository, directives repository.OperationDirectiveRepository, gates repository.GateUnitRepository, locks repository.GateExecutionLockRepository, security SecurityService) ExecutionConfirmationService {
	return &executionConfirmationService{repository: repo, directives: directives, gates: gates, locks: locks, security: security}
}

func (s *executionConfirmationService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.ExecutionConfirmation], error) {
	return s.repository.List(ctx, query)
}

func (s *executionConfirmationService) Get(ctx context.Context, id uint) (model.ExecutionConfirmation, error) {
	return s.repository.Get(ctx, id)
}

func (s *executionConfirmationService) Create(ctx context.Context, input dto.CreateExecutionConfirmation, actor, requestID string) (model.ExecutionConfirmation, error) {
	if err := validateExecutionConfirmationBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.ExecutionConfirmation{}, err
	}
	directive, err := s.requireExecutingDirective(ctx, input.RelatedCode)
	if err != nil {
		return model.ExecutionConfirmation{}, err
	}
	count, err := s.repository.CountByDirectiveCode(ctx, directive.Code)
	if err != nil {
		return model.ExecutionConfirmation{}, fmt.Errorf("check existing confirmation: %w", err)
	}
	if count > 0 {
		return model.ExecutionConfirmation{}, fmt.Errorf("%w: directive already has an execution confirmation", ErrInvalidInput)
	}
	item := model.ExecutionConfirmation{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.ExecutionConfirmationInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: directive.Facility, Owner: strings.TrimSpace(input.Owner),
		Category: directive.Category, RiskLevel: directive.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: directive.Code,
	}
	if err := s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Create(txCtx, &item); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "create", "ExecutionConfirmation", item.ID, "", item.Status, "created 执行确认")
	}); err != nil {
		return model.ExecutionConfirmation{}, fmt.Errorf("create 执行确认: %w", err)
	}
	return item, nil
}

func (s *executionConfirmationService) Update(ctx context.Context, id uint, input dto.UpdateExecutionConfirmation, actor, requestID string) (model.ExecutionConfirmation, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ExecutionConfirmation{}, err
	}
	if err := validateExecutionConfirmationBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.ExecutionConfirmation{}, err
	}
	if current.Status != model.ExecutionConfirmationInitialStatus {
		return model.ExecutionConfirmation{}, ErrImmutableState
	}
	if !strings.EqualFold(strings.TrimSpace(input.RelatedCode), current.RelatedCode) {
		return model.ExecutionConfirmation{}, fmt.Errorf("%w: linked directive cannot be changed", ErrInvalidInput)
	}
	directive, err := s.requireExecutingDirective(ctx, current.RelatedCode)
	if err != nil {
		return model.ExecutionConfirmation{}, err
	}
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = directive.Facility
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = directive.Category
	current.RiskLevel = directive.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.Evidence = strings.TrimSpace(input.Evidence)
	current.RelatedCode = directive.Code
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "update", "ExecutionConfirmation", id, current.Status, current.Status, "updated business fields")
	}); err != nil {
		return model.ExecutionConfirmation{}, fmt.Errorf("update 执行确认: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *executionConfirmationService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.ExecutionConfirmation, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ExecutionConfirmation{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.ExecutionConfirmationTransitions, current.Status, target) {
		return model.ExecutionConfirmation{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	now := time.Now().UTC()
	current.Status = target
	if target == "confirmed" {
		current.ConfirmedBy = actor
		current.ConfirmedAt = &now
	}
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = now

	var directive model.OperationDirective
	var gate model.GateUnit
	var directiveTarget, gateTarget string
	if before == model.ExecutionConfirmationInitialStatus {
		directive, err = s.requireExecutingDirective(ctx, current.RelatedCode)
		if err != nil {
			return model.ExecutionConfirmation{}, err
		}
		gate, err = s.gates.GetByCode(ctx, directive.RelatedCode)
		if err != nil {
			return model.ExecutionConfirmation{}, fmt.Errorf("linked gate %q: %w", directive.RelatedCode, err)
		}
		if target == "confirmed" {
			directiveTarget, gateTarget = string(constants.DirectiveStateCompleted), directive.GateState
		} else {
			directiveTarget, gateTarget = string(constants.DirectiveStateAborted), string(constants.GateStateLocked)
		}
		if gate.Status != gateTarget && !constants.CanTransition(constants.GateUnitTransitions, gate.Status, gateTarget) {
			return model.ExecutionConfirmation{}, fmt.Errorf("%w: gate %s cannot move from %s to %s", ErrInvalidTransition, gate.Code, gate.Status, gateTarget)
		}
	}

	if err := s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return err
		}
		if err := s.security.Audit(txCtx, actor, requestID, "transition", "ExecutionConfirmation", id, before, target, input.Reason); err != nil {
			return err
		}
		if directiveTarget == "" {
			return nil
		}
		directiveBefore := directive.Status
		directive.Status = directiveTarget
		directive.Version++
		directive.UpdatedAt = now
		if err := s.directives.Update(txCtx, directive.ID, directive.Version-1, &directive); err != nil {
			return err
		}
		if err := s.security.Audit(txCtx, actor, requestID, "execution_outcome", "OperationDirective", directive.ID, directiveBefore, directiveTarget, input.Reason); err != nil {
			return err
		}
		if gate.Status == gateTarget {
			return s.releaseExecutionLock(txCtx, directive, gate, actor, requestID, target)
		}
		gateBefore := gate.Status
		gate.Status = gateTarget
		gate.Version++
		gate.UpdatedAt = now
		if err := s.gates.Update(txCtx, gate.ID, gate.Version-1, &gate); err != nil {
			return err
		}
		if err := s.security.Audit(txCtx, actor, requestID, "execution_outcome", "GateUnit", gate.ID, gateBefore, gateTarget, input.Reason); err != nil {
			return err
		}
		return s.releaseExecutionLock(txCtx, directive, gate, actor, requestID, target)
	}); err != nil {
		return model.ExecutionConfirmation{}, fmt.Errorf("transition 执行确认: %w", err)
	}
	return s.repository.Get(ctx, id)
}

// releaseExecutionLock releases the gate execution right whenever an executing
// directive reaches its terminal state through a confirmation: a successful
// receipt (confirmed) or a failed receipt (failed). The delete is idempotent,
// and releasing is audited so the right never outlives the work it guarded.
func (s *executionConfirmationService) releaseExecutionLock(ctx context.Context, directive model.OperationDirective, gate model.GateUnit, actor, requestID, confirmationState string) error {
	released, err := s.locks.ReleaseByDirectiveID(ctx, directive.ID)
	if err != nil {
		return err
	}
	if !released {
		return nil
	}
	outcome := "完成"
	if confirmationState == "failed" {
		outcome = "回执失败"
	}
	return s.security.Audit(ctx, actor, requestID, "execution_lock_release", "GateUnit", gate.ID, "locked", "",
		fmt.Sprintf("指令 %s%s，释放闸门 %s 执行权", directive.Code, outcome, gate.Code))
}

func (s *executionConfirmationService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != model.ExecutionConfirmationInitialStatus {
		return ErrImmutableState
	}
	return s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Delete(txCtx, id); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "delete", "ExecutionConfirmation", id, current.Status, "deleted", "soft deleted 执行确认")
	})
}

func (s *executionConfirmationService) requireExecutingDirective(ctx context.Context, code string) (model.OperationDirective, error) {
	directive, err := s.directives.GetByCode(ctx, code)
	if err != nil {
		return model.OperationDirective{}, fmt.Errorf("linked directive %q: %w", code, err)
	}
	if directive.Status != string(constants.DirectiveStateExecuting) {
		return model.OperationDirective{}, fmt.Errorf("%w: execution confirmation requires an executing directive", ErrInvalidInput)
	}
	return directive, nil
}

func (s *executionConfirmationService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateExecutionConfirmationBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
