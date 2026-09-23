package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/config"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, cfg config.Config, log *slog.Logger) (*gorm.DB, *redis.Client, error) {
	var dialector gorm.Dialector
	switch cfg.DatabaseDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DatabaseDSN)
	case "mysql":
		dialector = mysql.Open(cfg.DatabaseDSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DatabaseDSN)
	default:
		return nil, nil, fmt.Errorf("unsupported database driver %q", cfg.DatabaseDriver)
	}
	logLevel := logger.Warn
	if cfg.Environment == "development" {
		logLevel = logger.Info
	}
	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= 20; attempt++ {
		db, err = gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logLevel)})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil && sqlDB.PingContext(ctx) == nil {
				break
			}
			if dbErr != nil {
				err = dbErr
			} else {
				err = sqlDB.PingContext(ctx)
			}
		}
		log.Warn("database not ready", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connect database: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, nil, err
	}
	if err := Seed(ctx, db, cfg); err != nil {
		return nil, nil, err
	}
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, nil, fmt.Errorf("connect redis: %w", err)
		}
	}
	return db, redisClient, nil
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{}, &model.AuditLog{},
		&model.Reservoir{},
		&model.GateUnit{},
		&model.OperationDirective{},
		&model.DirectiveApproval{},
		&model.ExecutionConfirmation{},
	)
}

func Seed(ctx context.Context, db *gorm.DB, cfg config.Config) error {
	var users int64
	if err := db.WithContext(ctx).Model(&model.User{}).Count(&users).Error; err != nil {
		return err
	}
	if users == 0 {
		accounts := []struct {
			username, displayName, role, password string
		}{
			{"admin", "系统管理员", model.RoleAdmin, cfg.BootstrapAdminPassword},
			{"operator", "现场操作员", model.RoleOperator, cfg.BootstrapOperatorPassword},
			{"reviewer", "安全复核员", model.RoleReviewer, cfg.BootstrapReviewerPassword},
		}
		if !cfg.IsProduction() {
			accounts = append(accounts, struct{ username, displayName, role, password string }{"viewer", "值班观察员", model.RoleViewer, cfg.BootstrapAdminPassword})
		}
		seedUsers := make([]model.User, 0, len(accounts))
		for _, account := range accounts {
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(account.password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			seedUsers = append(seedUsers, model.User{Username: account.username, DisplayName: account.displayName, PasswordHash: string(passwordHash), Role: account.role, Active: true})
		}
		if err := db.WithContext(ctx).Create(&seedUsers).Error; err != nil {
			return err
		}
	}

	if err := seedReservoir(ctx, db); err != nil {
		return err
	}

	if err := seedGateUnit(ctx, db); err != nil {
		return err
	}

	if err := seedOperationDirective(ctx, db); err != nil {
		return err
	}

	if err := seedExecutionConfirmation(ctx, db); err != nil {
		return err
	}

	return nil
}

func seedReservoir(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.Reservoir{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.Reservoir{

		{BaseModel: model.BaseModel{Code: "R-001", Name: "库区示例一", Status: "normal", Version: 1,
			Description: "用于启动验证和主要流程演示的库区记录"}, Facility: "水电站闸门调度许可区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "R-001"},

		{BaseModel: model.BaseModel{Code: "R-002", Name: "库区示例二", Status: "warning", Version: 1,
			Description: "用于启动验证和主要流程演示的库区记录"}, Facility: "水电站闸门调度许可区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "R-002"},

		{BaseModel: model.BaseModel{Code: "R-003", Name: "库区示例三", Status: "critical", Version: 1,
			Description: "用于启动验证和主要流程演示的库区记录"}, Facility: "水电站闸门调度许可区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "R-003"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedGateUnit(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.GateUnit{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.GateUnit{

		{BaseModel: model.BaseModel{Code: "GU-001", Name: "闸门示例一", Status: "open", Version: 1,
			Description: "用于启动验证和主要流程演示的闸门记录"}, Facility: "水电站闸门调度许可区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "R-001"},

		{BaseModel: model.BaseModel{Code: "GU-002", Name: "闸门示例二", Status: "closed", Version: 1,
			Description: "用于启动验证和主要流程演示的闸门记录"}, Facility: "水电站闸门调度许可区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "R-002"},

		{BaseModel: model.BaseModel{Code: "GU-003", Name: "闸门示例三", Status: "moving", Version: 1,
			Description: "用于启动验证和主要流程演示的闸门记录"}, Facility: "水电站闸门调度许可区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "R-003"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedOperationDirective(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.OperationDirective{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	submittedAt := now.Add(-2 * time.Hour)
	approvedAt := now.Add(-time.Hour)
	items := []model.OperationDirective{

		{BaseModel: model.BaseModel{Code: "OD-001", Name: "操作指令示例一", Status: "draft", Version: 1,
			Description: "用于启动验证和主要流程演示的操作指令记录"}, Facility: "水电站闸门调度许可区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已核对库水位与机组工况", RelatedCode: "GU-001", GateState: "open"},

		{BaseModel: model.BaseModel{Code: "OD-002", Name: "操作指令示例二", Status: "pending", Version: 1,
			Description: "用于启动验证和主要流程演示的操作指令记录"}, Facility: "水电站闸门调度许可区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "待安全复核员完成第二人确认", RelatedCode: "GU-002", GateState: "closed",
			SubmittedBy: "operator", SubmittedAt: &submittedAt},

		{BaseModel: model.BaseModel{Code: "OD-003", Name: "右岸泄洪闸开启指令", Status: "executing", Version: 1,
			Description: "用于启动验证和主要流程演示的操作指令记录"}, Facility: "水电站闸门调度许可区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "双人确认已完成，现场正在执行", RelatedCode: "GU-003", GateState: "open",
			SubmittedBy: "operator", SubmittedAt: &submittedAt, ApprovedBy: "reviewer", ApprovedAt: &approvedAt},
	}
	if err := db.WithContext(ctx).Create(&items).Error; err != nil {
		return err
	}
	approvals := []model.DirectiveApproval{
		{DirectiveID: items[1].ID, Stage: "submitted", Actor: "operator", Role: model.RoleOperator, RequestID: "seed-od-002-submit", Reason: "值班员提交调度许可", FromState: "draft", ToState: "pending", CreatedAt: submittedAt},
		{DirectiveID: items[2].ID, Stage: "submitted", Actor: "operator", Role: model.RoleOperator, RequestID: "seed-od-003-submit", Reason: "值班员提交调度许可", FromState: "draft", ToState: "pending", CreatedAt: submittedAt},
		{DirectiveID: items[2].ID, Stage: "approved", Actor: "reviewer", Role: model.RoleReviewer, RequestID: "seed-od-003-approve", Reason: "复核水位窗口和闸门目标无冲突", FromState: "pending", ToState: "approved", CreatedAt: approvedAt},
	}
	return db.WithContext(ctx).Create(&approvals).Error
}

func seedExecutionConfirmation(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.ExecutionConfirmation{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.ExecutionConfirmation{{
		BaseModel: model.BaseModel{Code: "EC-001", Name: "右岸泄洪闸现场执行回执", Status: "pending", Version: 1,
			Description: "关联已批准且正在执行的操作指令，记录现场反馈与证据"},
		Facility: "水电站闸门调度许可区域3", Owner: "运行一组", Category: "泄洪调度", RiskLevel: "high",
		MetricValue: 37.5, MetricUnit: "%", EffectiveAt: now, Evidence: "待现场核对开度反馈、视频与水位变化", RelatedCode: "OD-003",
	}}
	return db.WithContext(ctx).Create(&items).Error
}
