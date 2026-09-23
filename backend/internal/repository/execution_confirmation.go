package repository

import (
	"context"
	"strings"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/dto"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"gorm.io/gorm"
)

// ExecutionConfirmationRepository owns all persistence operations for 执行确认.
type ExecutionConfirmationRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.ExecutionConfirmation], error)
	Get(context.Context, uint) (model.ExecutionConfirmation, error)
	Create(context.Context, *model.ExecutionConfirmation) error
	Update(context.Context, uint, uint, *model.ExecutionConfirmation) error
	TransitionWithAudit(context.Context, uint, uint, *model.ExecutionConfirmation, *model.AuditLog) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
	CountByDirectiveCode(context.Context, string) (int64, error)
}

type executionConfirmationRepository struct {
	store *Store[model.ExecutionConfirmation]
	db    *gorm.DB
}

func NewExecutionConfirmationRepository(db *gorm.DB) ExecutionConfirmationRepository {
	return &executionConfirmationRepository{store: NewStore[model.ExecutionConfirmation](db), db: db}
}

func (r *executionConfirmationRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.ExecutionConfirmation], error) {
	return r.store.List(ctx, q)
}
func (r *executionConfirmationRepository) Get(ctx context.Context, id uint) (model.ExecutionConfirmation, error) {
	return r.store.Get(ctx, id)
}
func (r *executionConfirmationRepository) Create(ctx context.Context, item *model.ExecutionConfirmation) error {
	return r.store.Create(ctx, item)
}
func (r *executionConfirmationRepository) Update(ctx context.Context, id, version uint, item *model.ExecutionConfirmation) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *executionConfirmationRepository) TransitionWithAudit(ctx context.Context, id, version uint, item *model.ExecutionConfirmation, audit *model.AuditLog) error {
	return r.store.TransitionWithAudit(ctx, id, version, item, audit)
}
func (r *executionConfirmationRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *executionConfirmationRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
func (r *executionConfirmationRepository) CountByDirectiveCode(ctx context.Context, code string) (int64, error) {
	var count int64
	err := databaseForContext(ctx, r.db).Model(&model.ExecutionConfirmation{}).
		Where("related_code = ?", strings.ToUpper(strings.TrimSpace(code))).Count(&count).Error
	return count, err
}
