package repository

import (
	"context"
	"time"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"gorm.io/gorm"
)

type SecurityRepository interface {
	FindUserByUsername(context.Context, string) (model.User, error)
	CreateUser(context.Context, *model.User) error
	CountUsers(context.Context) (int64, error)
	AppendAudit(context.Context, *model.AuditLog) error
	ListAudits(context.Context, int, int, string) ([]model.AuditLog, int64, error)
	SummarizeAudits(context.Context, time.Time) (model.AuditSummary, error)
	EntityHistory(context.Context, string, uint, int) ([]model.AuditLog, error)
	WithinTransaction(context.Context, func(context.Context) error) error
}

type securityRepository struct{ db *gorm.DB }

func NewSecurityRepository(db *gorm.DB) SecurityRepository {
	return &securityRepository{db: db}
}

func (r *securityRepository) FindUserByUsername(ctx context.Context, username string) (model.User, error) {
	var user model.User
	err := databaseForContext(ctx, r.db).Where("username = ? AND active = ?", username, true).First(&user).Error
	return user, err
}

func (r *securityRepository) CreateUser(ctx context.Context, user *model.User) error {
	return databaseForContext(ctx, r.db).Create(user).Error
}

func (r *securityRepository) CountUsers(ctx context.Context) (int64, error) {
	var total int64
	return total, databaseForContext(ctx, r.db).Model(&model.User{}).Count(&total).Error
}

func (r *securityRepository) AppendAudit(ctx context.Context, log *model.AuditLog) error {
	return databaseForContext(ctx, r.db).Create(log).Error
}

func (r *securityRepository) ListAudits(ctx context.Context, page, pageSize int, search string) ([]model.AuditLog, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	db := databaseForContext(ctx, r.db).Model(&model.AuditLog{})
	if search != "" {
		wildcard := "%" + search + "%"
		db = db.Where("actor LIKE ? OR entity_type LIKE ? OR action LIKE ?", wildcard, wildcard, wildcard)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	logs := make([]model.AuditLog, 0)
	err := db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

func (r *securityRepository) SummarizeAudits(ctx context.Context, since time.Time) (model.AuditSummary, error) {
	summary := model.AuditSummary{
		Since: since.UTC(), Actions: make([]model.AuditActionCount, 0),
		EntityTypes: make([]model.AuditEntityCount, 0),
	}
	base := databaseForContext(ctx, r.db).Model(&model.AuditLog{}).Where("created_at >= ?", since.UTC())
	if err := base.Count(&summary.Total).Error; err != nil {
		return model.AuditSummary{}, err
	}
	if err := databaseForContext(ctx, r.db).Model(&model.AuditLog{}).
		Where("created_at >= ? AND action = ?", since.UTC(), "transition").Count(&summary.Transitions).Error; err != nil {
		return model.AuditSummary{}, err
	}
	if err := databaseForContext(ctx, r.db).Model(&model.AuditLog{}).
		Where("created_at >= ?", since.UTC()).Distinct("actor").Count(&summary.UniqueActors).Error; err != nil {
		return model.AuditSummary{}, err
	}
	if err := databaseForContext(ctx, r.db).Model(&model.AuditLog{}).
		Select("action, COUNT(*) AS count").Where("created_at >= ?", since.UTC()).
		Group("action").Order("count DESC").Scan(&summary.Actions).Error; err != nil {
		return model.AuditSummary{}, err
	}
	if err := databaseForContext(ctx, r.db).Model(&model.AuditLog{}).
		Select("entity_type, COUNT(*) AS count").Where("created_at >= ?", since.UTC()).
		Group("entity_type").Order("count DESC").Scan(&summary.EntityTypes).Error; err != nil {
		return model.AuditSummary{}, err
	}
	return summary, nil
}

func (r *securityRepository) EntityHistory(ctx context.Context, entityType string, entityID uint, limit int) ([]model.AuditLog, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	logs := make([]model.AuditLog, 0, limit)
	err := databaseForContext(ctx, r.db).Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at DESC").Limit(limit).Find(&logs).Error
	return logs, err
}

func (r *securityRepository) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return withinTransaction(ctx, r.db, fn)
}
