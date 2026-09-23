package repository

import (
	"context"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"gorm.io/gorm"
)

// GateExecutionLockRepository owns the execution right rows that bind a gate to
// at most one running directive. It deliberately exposes no update operation:
// a lock is either acquired or released.
type GateExecutionLockRepository interface {
	Acquire(context.Context, *model.GateExecutionLock) error
	ReleaseByDirectiveID(context.Context, uint) (bool, error)
	ActiveByGateIDs(context.Context, []uint) (map[uint]model.GateExecutionLock, error)
	ActiveByDirectiveIDs(context.Context, []uint) (map[uint]model.GateExecutionLock, error)
	ActiveByGateCode(context.Context, string) (model.GateExecutionLock, error)
	ActiveByGateID(context.Context, uint) (model.GateExecutionLock, error)
	ActiveByDirectiveID(context.Context, uint) (model.GateExecutionLock, error)
	ListActive(context.Context) ([]model.GateExecutionLock, error)
}

type gateExecutionLockRepository struct{ db *gorm.DB }

func NewGateExecutionLockRepository(db *gorm.DB) GateExecutionLockRepository {
	return &gateExecutionLockRepository{db: db}
}

// Acquire relies on the unique gate index: two concurrent transactions can
// never insert a lock for the same gate. Callers re-read the active lock when
// the insert fails to identify the directive holding the execution right.
func (r *gateExecutionLockRepository) Acquire(ctx context.Context, lock *model.GateExecutionLock) error {
	return databaseForContext(ctx, r.db).Create(lock).Error
}

func (r *gateExecutionLockRepository) ReleaseByDirectiveID(ctx context.Context, directiveID uint) (bool, error) {
	result := databaseForContext(ctx, r.db).
		Where("directive_id = ?", directiveID).
		Delete(&model.GateExecutionLock{})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *gateExecutionLockRepository) ActiveByGateIDs(ctx context.Context, ids []uint) (map[uint]model.GateExecutionLock, error) {
	locks := make(map[uint]model.GateExecutionLock)
	if len(ids) == 0 {
		return locks, nil
	}
	var rows []model.GateExecutionLock
	if err := databaseForContext(ctx, r.db).Where("gate_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		locks[row.GateID] = row
	}
	return locks, nil
}

func (r *gateExecutionLockRepository) ActiveByDirectiveIDs(ctx context.Context, ids []uint) (map[uint]model.GateExecutionLock, error) {
	locks := make(map[uint]model.GateExecutionLock)
	if len(ids) == 0 {
		return locks, nil
	}
	var rows []model.GateExecutionLock
	if err := databaseForContext(ctx, r.db).Where("directive_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		locks[row.DirectiveID] = row
	}
	return locks, nil
}

func (r *gateExecutionLockRepository) ActiveByGateCode(ctx context.Context, code string) (model.GateExecutionLock, error) {
	var lock model.GateExecutionLock
	err := databaseForContext(ctx, r.db).Where("gate_code = ?", code).First(&lock).Error
	return lock, err
}

func (r *gateExecutionLockRepository) ActiveByGateID(ctx context.Context, gateID uint) (model.GateExecutionLock, error) {
	var lock model.GateExecutionLock
	err := databaseForContext(ctx, r.db).Where("gate_id = ?", gateID).First(&lock).Error
	return lock, err
}

func (r *gateExecutionLockRepository) ActiveByDirectiveID(ctx context.Context, directiveID uint) (model.GateExecutionLock, error) {
	var lock model.GateExecutionLock
	err := databaseForContext(ctx, r.db).Where("directive_id = ?", directiveID).First(&lock).Error
	return lock, err
}

func (r *gateExecutionLockRepository) ListActive(ctx context.Context) ([]model.GateExecutionLock, error) {
	var rows []model.GateExecutionLock
	err := databaseForContext(ctx, r.db).Order("acquired_at ASC, id ASC").Find(&rows).Error
	return rows, err
}
