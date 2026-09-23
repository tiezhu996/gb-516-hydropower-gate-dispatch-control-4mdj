package repository

import (
	"context"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/dto"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"gorm.io/gorm"
)

// ReservoirRepository owns all persistence operations for 库区.
type ReservoirRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.Reservoir], error)
	Get(context.Context, uint) (model.Reservoir, error)
	GetByCode(context.Context, string) (model.Reservoir, error)
	Create(context.Context, *model.Reservoir) error
	Update(context.Context, uint, uint, *model.Reservoir) error
	TransitionWithAudit(context.Context, uint, uint, *model.Reservoir, *model.AuditLog) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type reservoirRepository struct {
	store *Store[model.Reservoir]
}

func NewReservoirRepository(db *gorm.DB) ReservoirRepository {
	return &reservoirRepository{store: NewStore[model.Reservoir](db)}
}

func (r *reservoirRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.Reservoir], error) {
	return r.store.List(ctx, q)
}
func (r *reservoirRepository) Get(ctx context.Context, id uint) (model.Reservoir, error) {
	return r.store.Get(ctx, id)
}
func (r *reservoirRepository) GetByCode(ctx context.Context, code string) (model.Reservoir, error) {
	return r.store.GetByCode(ctx, code)
}
func (r *reservoirRepository) Create(ctx context.Context, item *model.Reservoir) error {
	return r.store.Create(ctx, item)
}
func (r *reservoirRepository) Update(ctx context.Context, id, version uint, item *model.Reservoir) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *reservoirRepository) TransitionWithAudit(ctx context.Context, id, version uint, item *model.Reservoir, audit *model.AuditLog) error {
	return r.store.TransitionWithAudit(ctx, id, version, item, audit)
}
func (r *reservoirRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *reservoirRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
