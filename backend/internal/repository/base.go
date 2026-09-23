package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/dto"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"gorm.io/gorm"
)

var ErrVersionConflict = errors.New("record was changed by another request")

type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// Store centralizes consistent paging and optimistic-lock semantics while
// concrete repository files retain an explicit boundary for each aggregate.
type Store[T any] struct {
	db *gorm.DB
}

type transactionContextKey struct{}

func databaseForContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(transactionContextKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return fallback.WithContext(ctx)
}

func withinTransaction(ctx context.Context, db *gorm.DB, fn func(context.Context) error) error {
	if _, ok := ctx.Value(transactionContextKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, transactionContextKey{}, tx))
	})
}

func NewStore[T any](db *gorm.DB) *Store[T] { return &Store[T]{db: db} }

func (s *Store[T]) List(ctx context.Context, query dto.PageQuery) (Page[T], error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	db := databaseForContext(ctx, s.db).Model(new(T))
	if search := strings.TrimSpace(strings.ToLower(query.Search)); search != "" {
		wildcard := "%" + search + "%"
		db = db.Where("LOWER(code) LIKE ? OR LOWER(name) LIKE ?", wildcard, wildcard)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return Page[T]{}, err
	}
	items := make([]T, 0)
	err := db.Order("updated_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return Page[T]{Items: items, Total: total, Page: page, PageSize: pageSize}, err
}

func (s *Store[T]) Get(ctx context.Context, id uint) (T, error) {
	var item T
	err := databaseForContext(ctx, s.db).First(&item, id).Error
	return item, err
}

func (s *Store[T]) GetByCode(ctx context.Context, code string) (T, error) {
	var item T
	err := databaseForContext(ctx, s.db).Where("code = ?", strings.ToUpper(strings.TrimSpace(code))).First(&item).Error
	return item, err
}

func (s *Store[T]) Create(ctx context.Context, item *T) error {
	return databaseForContext(ctx, s.db).Create(item).Error
}

func (s *Store[T]) Update(ctx context.Context, id, expectedVersion uint, item *T) error {
	return s.update(databaseForContext(ctx, s.db), id, expectedVersion, item)
}

func (s *Store[T]) update(db *gorm.DB, id, expectedVersion uint, item *T) error {
	result := db.Model(new(T)).
		Where("id = ? AND version = ?", id, expectedVersion).
		Select("*").Omit("id", "code", "created_at", "deleted_at").Updates(item)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}

// TransitionWithAudit makes the state update and its immutable audit record a
// single commit. A dispatch state must never advance without evidence.
func (s *Store[T]) TransitionWithAudit(ctx context.Context, id, expectedVersion uint, item *T, audit *model.AuditLog) error {
	return databaseForContext(ctx, s.db).Transaction(func(tx *gorm.DB) error {
		if err := s.update(tx, id, expectedVersion, item); err != nil {
			return err
		}
		return tx.Create(audit).Error
	})
}

func (s *Store[T]) Delete(ctx context.Context, id uint) error {
	result := databaseForContext(ctx, s.db).Delete(new(T), id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store[T]) CountByStatus(ctx context.Context) (map[string]int64, error) {
	rows, err := databaseForContext(ctx, s.db).Model(new(T)).
		Select("status, COUNT(*) AS total").Group("status").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[string]int64)
	for rows.Next() {
		var status string
		var total int64
		if err := rows.Scan(&status, &total); err != nil {
			return nil, err
		}
		counts[status] = total
	}
	return counts, rows.Err()
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
