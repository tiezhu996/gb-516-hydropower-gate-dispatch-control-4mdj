package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/constants"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/dto"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/repository"
	"gorm.io/gorm"
)

type OperationDirectiveService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.OperationDirective], error)
	Get(context.Context, uint) (model.OperationDirective, error)
	Create(context.Context, dto.CreateOperationDirective, string, string) (model.OperationDirective, error)
	Update(context.Context, uint, dto.UpdateOperationDirective, string, string) (model.OperationDirective, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string, string) (model.OperationDirective, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type operationDirectiveService struct {
	repository repository.OperationDirectiveRepository
	gates      repository.GateUnitRepository
	locks      repository.GateExecutionLockRepository
	security   SecurityService
}

func NewOperationDirectiveService(repo repository.OperationDirectiveRepository, gates repository.GateUnitRepository, locks repository.GateExecutionLockRepository, security SecurityService) OperationDirectiveService {
	return &operationDirectiveService{repository: repo, gates: gates, locks: locks, security: security}
}

func (s *operationDirectiveService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.OperationDirective], error) {
	page, err := s.repository.List(ctx, query)
	if err != nil {
		return page, err
	}
	if err := s.attachExecutionLocks(ctx, page.Items); err != nil {
		return repository.Page[model.OperationDirective]{}, err
	}
	return page, nil
}

func (s *operationDirectiveService) Get(ctx context.Context, id uint) (model.OperationDirective, error) {
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.OperationDirective{}, err
	}
	lock, err := s.locks.ActiveByDirectiveID(ctx, id)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.OperationDirective{}, err
	}
	if err == nil {
		item.ExecutionLock = &lock
	}
	return item, nil
}

// attachExecutionLocks fills the transient execution-right marker in one query
// so list pages can show which directive currently occupies each gate.
func (s *operationDirectiveService) attachExecutionLocks(ctx context.Context, items []model.OperationDirective) error {
	ids := make([]uint, 0, len(items))
	for index := range items {
		if items[index].Status == string(constants.DirectiveStateExecuting) {
			ids = append(ids, items[index].ID)
		}
	}
	locks, err := s.locks.ActiveByDirectiveIDs(ctx, ids)
	if err != nil {
		return err
	}
	for index := range items {
		if lock, ok := locks[items[index].ID]; ok {
			lockCopy := lock
			items[index].ExecutionLock = &lockCopy
		}
	}
	return nil
}

func (s *operationDirectiveService) Create(ctx context.Context, input dto.CreateOperationDirective, actor, requestID string) (model.OperationDirective, error) {
	if err := validateOperationDirectiveBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.OperationDirective{}, err
	}
	gate, err := s.gates.GetByCode(ctx, input.RelatedCode)
	if err != nil {
		return model.OperationDirective{}, fmt.Errorf("linked gate %q: %w", input.RelatedCode, err)
	}
	if !strings.EqualFold(strings.TrimSpace(input.Facility), gate.Facility) || gate.Status == string(constants.GateStateLocked) {
		return model.OperationDirective{}, fmt.Errorf("%w: linked gate is locked or belongs to another facility", ErrInvalidInput)
	}
	gateState := strings.TrimSpace(input.GateState)
	if gateState == "" {
		gateState = string(constants.GateStateClosed)
	}
	item := model.OperationDirective{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.OperationDirectiveInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)), GateState: gateState,
	}
	if err := s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Create(txCtx, &item); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "create", "OperationDirective", item.ID, "", item.Status, "created 操作指令")
	}); err != nil {
		return model.OperationDirective{}, fmt.Errorf("create 操作指令: %w", err)
	}
	return item, nil
}

func (s *operationDirectiveService) Update(ctx context.Context, id uint, input dto.UpdateOperationDirective, actor, requestID string) (model.OperationDirective, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.OperationDirective{}, err
	}
	if current.Status != string(constants.DirectiveStateDraft) {
		return model.OperationDirective{}, ErrImmutableState
	}
	if err := validateOperationDirectiveBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.OperationDirective{}, err
	}
	gate, err := s.gates.GetByCode(ctx, input.RelatedCode)
	if err != nil {
		return model.OperationDirective{}, fmt.Errorf("linked gate %q: %w", input.RelatedCode, err)
	}
	if !strings.EqualFold(strings.TrimSpace(input.Facility), gate.Facility) || gate.Status == string(constants.GateStateLocked) {
		return model.OperationDirective{}, fmt.Errorf("%w: linked gate is locked or belongs to another facility", ErrInvalidInput)
	}
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = strings.TrimSpace(input.Facility)
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = strings.TrimSpace(input.Category)
	current.RiskLevel = input.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.Evidence = strings.TrimSpace(input.Evidence)
	current.RelatedCode = strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	if gateState := strings.TrimSpace(input.GateState); gateState != "" {
		current.GateState = gateState
	}
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "update", "OperationDirective", id, current.Status, current.Status, "updated business fields")
	}); err != nil {
		return model.OperationDirective{}, fmt.Errorf("update 操作指令: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *operationDirectiveService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, role, requestID string) (model.OperationDirective, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.OperationDirective{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.OperationDirectiveTransitions, current.Status, target) {
		return model.OperationDirective{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	if target == string(constants.DirectiveStateCompleted) {
		return model.OperationDirective{}, fmt.Errorf("%w: completion is recorded by an execution confirmation", ErrInvalidTransition)
	}
	if !directiveRoleAllowed(current.Status, target, role) {
		return model.OperationDirective{}, ErrForbidden
	}
	var gate *model.GateUnit
	var gateTarget string
	acquireLock := target == string(constants.DirectiveStateExecuting)
	releaseLock := current.Status == string(constants.DirectiveStateExecuting) && target == string(constants.DirectiveStateAborted)
	if acquireLock || releaseLock {
		linkedGate, gateErr := s.gates.GetByCode(ctx, current.RelatedCode)
		if gateErr != nil {
			return model.OperationDirective{}, fmt.Errorf("linked gate %q: %w", current.RelatedCode, gateErr)
		}
		if acquireLock && linkedGate.Status == string(constants.GateStateLocked) {
			return model.OperationDirective{}, fmt.Errorf("%w: locked gate cannot execute a directive", ErrInvalidInput)
		}
		gate = &linkedGate
		if releaseLock {
			gateTarget = string(constants.GateStateLocked)
		} else if linkedGate.Status != current.GateState {
			gateTarget = string(constants.GateStateMoving)
		}
		if gateTarget != "" && linkedGate.Status != gateTarget && !constants.CanTransition(constants.GateUnitTransitions, linkedGate.Status, gateTarget) {
			return model.OperationDirective{}, fmt.Errorf("%w: gate %s cannot move from %s to %s", ErrInvalidTransition, linkedGate.Code, linkedGate.Status, gateTarget)
		}
	}
	// Fast-fail when the execution right is visibly held by another approved
	// directive. The unique lock index inside the transaction remains the
	// authoritative guard for requests that start back-to-back.
	if acquireLock {
		if occupied, lockErr := s.locks.ActiveByGateCode(ctx, current.RelatedCode); lockErr == nil {
			if occupied.DirectiveID != id {
				return model.OperationDirective{}, &GateOccupiedError{GateCode: current.RelatedCode, OccupiedDirectiveCode: occupied.DirectiveCode}
			}
		} else if !errors.Is(lockErr, gorm.ErrRecordNotFound) {
			return model.OperationDirective{}, fmt.Errorf("check execution right on gate %q: %w", current.RelatedCode, lockErr)
		}
	}
	if target == string(constants.DirectiveStateApproved) {
		if current.SubmittedBy == "" || strings.EqualFold(current.SubmittedBy, actor) {
			return model.OperationDirective{}, ErrTwoPersonRequired
		}
	}
	before := current.Status
	now := time.Now().UTC()
	current.Status = target
	var approval *model.DirectiveApproval
	if target == string(constants.DirectiveStatePending) {
		current.SubmittedBy = actor
		current.SubmittedAt = &now
		approval = &model.DirectiveApproval{Stage: "submitted", Actor: actor, Role: role, RequestID: requestID,
			Reason: strings.TrimSpace(input.Reason), FromState: before, ToState: target, CreatedAt: now}
	}
	if target == string(constants.DirectiveStateApproved) {
		current.ApprovedBy = actor
		current.ApprovedAt = &now
		approval = &model.DirectiveApproval{Stage: "approved", Actor: actor, Role: role, RequestID: requestID,
			Reason: strings.TrimSpace(input.Reason), FromState: before, ToState: target, CreatedAt: now}
	}
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = now
	audit := &model.AuditLog{Actor: actor, RequestID: requestID, Action: "transition", EntityType: "OperationDirective", EntityID: id,
		BeforeState: before, AfterState: target, Detail: input.Reason, CreatedAt: now}
	txErr := s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if acquireLock {
			// Acquire the gate execution right first so the unique index fails
			// the whole commit when a concurrent request already owns the gate;
			// directive status, gate state and audit then advance atomically.
			if err := s.locks.Acquire(txCtx, &model.GateExecutionLock{
				GateID: gate.ID, GateCode: gate.Code, DirectiveID: id, DirectiveCode: current.Code,
				AcquiredBy: actor, RequestID: requestID, AcquiredAt: now,
			}); err != nil {
				return err
			}
			if err := s.security.Audit(txCtx, actor, requestID, "execution_lock_acquire", "GateUnit", gate.ID, "", "locked",
				fmt.Sprintf("指令 %s 获取闸门 %s 执行权", current.Code, gate.Code)); err != nil {
				return err
			}
		}
		if err := s.repository.TransitionWithApproval(txCtx, id, input.ExpectedVersion, &current, approval, audit); err != nil {
			return err
		}
		if gate != nil && gateTarget != "" && gate.Status != gateTarget {
			gateBefore := gate.Status
			gate.Status = gateTarget
			gate.Version++
			gate.UpdatedAt = now
			if err := s.gates.Update(txCtx, gate.ID, gate.Version-1, gate); err != nil {
				return err
			}
			if err := s.security.Audit(txCtx, actor, requestID, "directive_execution", "GateUnit", gate.ID, gateBefore, gateTarget, input.Reason); err != nil {
				return err
			}
		}
		if releaseLock {
			released, err := s.locks.ReleaseByDirectiveID(txCtx, id)
			if err != nil {
				return err
			}
			if released {
				if err := s.security.Audit(txCtx, actor, requestID, "execution_lock_release", "GateUnit", gate.ID, "locked", "",
					fmt.Sprintf("指令 %s 中止，释放闸门 %s 执行权", current.Code, gate.Code)); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if txErr != nil {
		if acquireLock {
			// Another directive won the gate: name the holder so the field team
			// keeps the loser unchanged. When this directive already owns the
			// right it is a back-to-back duplicate of the same start request,
			// which is an optimistic-lock conflict rather than an occupation.
			occupied, lockErr := s.locks.ActiveByGateCode(ctx, current.RelatedCode)
			switch {
			case lockErr == nil && occupied.DirectiveID != id:
				return model.OperationDirective{}, &GateOccupiedError{GateCode: current.RelatedCode, OccupiedDirectiveCode: occupied.DirectiveCode}
			case lockErr == nil && occupied.DirectiveID == id:
				return model.OperationDirective{}, fmt.Errorf("directive %s already holds the execution right: %w", current.Code, repository.ErrVersionConflict)
			}
		}
		return model.OperationDirective{}, fmt.Errorf("transition 操作指令: %w", txErr)
	}
	return s.Get(ctx, id)
}

func (s *operationDirectiveService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != string(constants.DirectiveStateDraft) {
		return ErrImmutableState
	}
	return s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Delete(txCtx, id); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "delete", "OperationDirective", id, current.Status, "deleted", "soft deleted 操作指令")
	})
}

func directiveRoleAllowed(from, target, role string) bool {
	switch target {
	case string(constants.DirectiveStatePending):
		return role == model.RoleOperator || role == model.RoleAdmin
	case string(constants.DirectiveStateApproved):
		return role == model.RoleReviewer || role == model.RoleAdmin
	case string(constants.DirectiveStateExecuting):
		return role == model.RoleOperator || role == model.RoleAdmin
	case string(constants.DirectiveStateAborted):
		return role == model.RoleOperator || role == model.RoleReviewer || role == model.RoleAdmin
	default:
		return false
	}
}

func (s *operationDirectiveService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateOperationDirectiveBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
