package repository

import (
	"context"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/dto"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"gorm.io/gorm"
)

// GateUnitRepository owns all persistence operations for 闸门.
type GateUnitRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.GateUnit], error)
	Get(context.Context, uint) (model.GateUnit, error)
	GetByCode(context.Context, string) (model.GateUnit, error)
	Create(context.Context, *model.GateUnit) error
	Update(context.Context, uint, uint, *model.GateUnit) error
	TransitionWithAudit(context.Context, uint, uint, *model.GateUnit, *model.AuditLog) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type gateUnitRepository struct {
	store *Store[model.GateUnit]
}

func NewGateUnitRepository(db *gorm.DB) GateUnitRepository {
	return &gateUnitRepository{store: NewStore[model.GateUnit](db)}
}

func (r *gateUnitRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.GateUnit], error) {
	return r.store.List(ctx, q)
}
func (r *gateUnitRepository) Get(ctx context.Context, id uint) (model.GateUnit, error) {
	return r.store.Get(ctx, id)
}
func (r *gateUnitRepository) GetByCode(ctx context.Context, code string) (model.GateUnit, error) {
	return r.store.GetByCode(ctx, code)
}
func (r *gateUnitRepository) Create(ctx context.Context, item *model.GateUnit) error {
	return r.store.Create(ctx, item)
}
func (r *gateUnitRepository) Update(ctx context.Context, id, version uint, item *model.GateUnit) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *gateUnitRepository) TransitionWithAudit(ctx context.Context, id, version uint, item *model.GateUnit, audit *model.AuditLog) error {
	return r.store.TransitionWithAudit(ctx, id, version, item, audit)
}
func (r *gateUnitRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *gateUnitRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
