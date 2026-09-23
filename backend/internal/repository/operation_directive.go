package repository

import (
	"context"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/dto"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"gorm.io/gorm"
)

// OperationDirectiveRepository owns all persistence operations for 操作指令.
type OperationDirectiveRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.OperationDirective], error)
	Get(context.Context, uint) (model.OperationDirective, error)
	GetByCode(context.Context, string) (model.OperationDirective, error)
	Create(context.Context, *model.OperationDirective) error
	Update(context.Context, uint, uint, *model.OperationDirective) error
	TransitionWithApproval(context.Context, uint, uint, *model.OperationDirective, *model.DirectiveApproval, *model.AuditLog) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type operationDirectiveRepository struct {
	store *Store[model.OperationDirective]
	db    *gorm.DB
}

func NewOperationDirectiveRepository(db *gorm.DB) OperationDirectiveRepository {
	return &operationDirectiveRepository{store: NewStore[model.OperationDirective](db), db: db}
}

func (r *operationDirectiveRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.OperationDirective], error) {
	page, err := r.store.List(ctx, q)
	if err != nil {
		return Page[model.OperationDirective]{}, err
	}
	for index := range page.Items {
		if err := databaseForContext(ctx, r.db).Where("directive_id = ?", page.Items[index].ID).
			Order("created_at ASC, id ASC").Find(&page.Items[index].Approvals).Error; err != nil {
			return Page[model.OperationDirective]{}, err
		}
	}
	return page, nil
}
func (r *operationDirectiveRepository) Get(ctx context.Context, id uint) (model.OperationDirective, error) {
	var item model.OperationDirective
	err := databaseForContext(ctx, r.db).Preload("Approvals", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC, id ASC")
	}).First(&item, id).Error
	return item, err
}
func (r *operationDirectiveRepository) GetByCode(ctx context.Context, code string) (model.OperationDirective, error) {
	item, err := r.store.GetByCode(ctx, code)
	if err != nil {
		return model.OperationDirective{}, err
	}
	return r.Get(ctx, item.ID)
}
func (r *operationDirectiveRepository) Create(ctx context.Context, item *model.OperationDirective) error {
	return r.store.Create(ctx, item)
}
func (r *operationDirectiveRepository) Update(ctx context.Context, id, version uint, item *model.OperationDirective) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *operationDirectiveRepository) TransitionWithApproval(ctx context.Context, id, version uint, item *model.OperationDirective, approval *model.DirectiveApproval, audit *model.AuditLog) error {
	return databaseForContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if err := r.store.update(tx, id, version, item); err != nil {
			return err
		}
		if approval != nil {
			approval.DirectiveID = id
			if err := tx.Create(approval).Error; err != nil {
				return err
			}
		}
		return tx.Create(audit).Error
	})
}
func (r *operationDirectiveRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *operationDirectiveRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
