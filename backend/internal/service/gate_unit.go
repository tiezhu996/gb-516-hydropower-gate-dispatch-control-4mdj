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

type GateUnitService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.GateUnit], error)
	Get(context.Context, uint) (model.GateUnit, error)
	Create(context.Context, dto.CreateGateUnit, string, string) (model.GateUnit, error)
	Update(context.Context, uint, dto.UpdateGateUnit, string, string) (model.GateUnit, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.GateUnit, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type gateUnitService struct {
	repository repository.GateUnitRepository
	reservoirs repository.ReservoirRepository
	locks      repository.GateExecutionLockRepository
	security   SecurityService
}

func NewGateUnitService(repo repository.GateUnitRepository, reservoirs repository.ReservoirRepository, locks repository.GateExecutionLockRepository, security SecurityService) GateUnitService {
	return &gateUnitService{repository: repo, reservoirs: reservoirs, locks: locks, security: security}
}

func (s *gateUnitService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.GateUnit], error) {
	page, err := s.repository.List(ctx, query)
	if err != nil {
		return page, err
	}
	ids := make([]uint, 0, len(page.Items))
	for index := range page.Items {
		ids = append(ids, page.Items[index].ID)
	}
	locks, err := s.locks.ActiveByGateIDs(ctx, ids)
	if err != nil {
		return repository.Page[model.GateUnit]{}, err
	}
	for index := range page.Items {
		if lock, ok := locks[page.Items[index].ID]; ok {
			lockCopy := lock
			page.Items[index].ExecutionLock = &lockCopy
		}
	}
	return page, nil
}

func (s *gateUnitService) Get(ctx context.Context, id uint) (model.GateUnit, error) {
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.GateUnit{}, err
	}
	lock, err := s.locks.ActiveByGateID(ctx, id)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.GateUnit{}, err
	}
	if err == nil {
		item.ExecutionLock = &lock
	}
	return item, nil
}

func (s *gateUnitService) Create(ctx context.Context, input dto.CreateGateUnit, actor, requestID string) (model.GateUnit, error) {
	if err := validateGateUnitBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.GateUnit{}, err
	}
	reservoir, err := s.reservoirs.GetByCode(ctx, input.RelatedCode)
	if err != nil {
		return model.GateUnit{}, fmt.Errorf("linked reservoir %q: %w", input.RelatedCode, err)
	}
	if !strings.EqualFold(strings.TrimSpace(input.Facility), reservoir.Facility) {
		return model.GateUnit{}, fmt.Errorf("%w: gate and reservoir must belong to the same facility", ErrInvalidInput)
	}
	item := model.GateUnit{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.GateUnitInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Create(txCtx, &item); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "create", "GateUnit", item.ID, "", item.Status, "created 闸门")
	}); err != nil {
		return model.GateUnit{}, fmt.Errorf("create 闸门: %w", err)
	}
	return item, nil
}

func (s *gateUnitService) Update(ctx context.Context, id uint, input dto.UpdateGateUnit, actor, requestID string) (model.GateUnit, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.GateUnit{}, err
	}
	if err := validateGateUnitBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.GateUnit{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(input.RelatedCode), current.RelatedCode) {
		return model.GateUnit{}, fmt.Errorf("%w: linked reservoir cannot be changed", ErrInvalidInput)
	}
	reservoir, err := s.reservoirs.GetByCode(ctx, current.RelatedCode)
	if err != nil {
		return model.GateUnit{}, fmt.Errorf("linked reservoir %q: %w", current.RelatedCode, err)
	}
	if !strings.EqualFold(strings.TrimSpace(input.Facility), reservoir.Facility) {
		return model.GateUnit{}, fmt.Errorf("%w: gate and reservoir must belong to the same facility", ErrInvalidInput)
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
	current.RelatedCode = reservoir.Code
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "update", "GateUnit", id, current.Status, current.Status, "updated business fields")
	}); err != nil {
		return model.GateUnit{}, fmt.Errorf("update 闸门: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *gateUnitService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.GateUnit, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.GateUnit{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.GateUnitTransitions, current.Status, target) {
		return model.GateUnit{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	audit := &model.AuditLog{Actor: actor, RequestID: requestID, Action: "transition", EntityType: "GateUnit", EntityID: id,
		BeforeState: before, AfterState: target, Detail: input.Reason, CreatedAt: time.Now().UTC()}
	if err := s.repository.TransitionWithAudit(ctx, id, input.ExpectedVersion, &current, audit); err != nil {
		return model.GateUnit{}, fmt.Errorf("transition 闸门: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *gateUnitService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Delete(txCtx, id); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "delete", "GateUnit", id, current.Status, "deleted", "soft deleted 闸门")
	})
}

func (s *gateUnitService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateGateUnitBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
