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

type ReservoirService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.Reservoir], error)
	Get(context.Context, uint) (model.Reservoir, error)
	Create(context.Context, dto.CreateReservoir, string, string) (model.Reservoir, error)
	Update(context.Context, uint, dto.UpdateReservoir, string, string) (model.Reservoir, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.Reservoir, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type reservoirService struct {
	repository repository.ReservoirRepository
	security   SecurityService
}

func NewReservoirService(repo repository.ReservoirRepository, security SecurityService) ReservoirService {
	return &reservoirService{repository: repo, security: security}
}

func (s *reservoirService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.Reservoir], error) {
	return s.repository.List(ctx, query)
}

func (s *reservoirService) Get(ctx context.Context, id uint) (model.Reservoir, error) {
	return s.repository.Get(ctx, id)
}

func (s *reservoirService) Create(ctx context.Context, input dto.CreateReservoir, actor, requestID string) (model.Reservoir, error) {
	if err := validateReservoirBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.Reservoir{}, err
	}
	item := model.Reservoir{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.ReservoirInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
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
		return s.security.Audit(txCtx, actor, requestID, "create", "Reservoir", item.ID, "", item.Status, "created 库区")
	}); err != nil {
		return model.Reservoir{}, fmt.Errorf("create 库区: %w", err)
	}
	return item, nil
}

func (s *reservoirService) Update(ctx context.Context, id uint, input dto.UpdateReservoir, actor, requestID string) (model.Reservoir, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.Reservoir{}, err
	}
	if err := validateReservoirBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.Reservoir{}, err
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
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "update", "Reservoir", id, current.Status, current.Status, "updated business fields")
	}); err != nil {
		return model.Reservoir{}, fmt.Errorf("update 库区: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *reservoirService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.Reservoir, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.Reservoir{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.ReservoirTransitions, current.Status, target) {
		return model.Reservoir{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	audit := &model.AuditLog{Actor: actor, RequestID: requestID, Action: "transition", EntityType: "Reservoir", EntityID: id,
		BeforeState: before, AfterState: target, Detail: input.Reason, CreatedAt: time.Now().UTC()}
	if err := s.repository.TransitionWithAudit(ctx, id, input.ExpectedVersion, &current, audit); err != nil {
		return model.Reservoir{}, fmt.Errorf("transition 库区: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *reservoirService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.security.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.Delete(txCtx, id); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "delete", "Reservoir", id, current.Status, "deleted", "soft deleted 库区")
	})
}

func (s *reservoirService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateReservoirBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
