package service

import (
	"context"
	"fmt"
	"time"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/config"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/dto"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type SecurityService interface {
	Login(context.Context, dto.LoginRequest) (dto.LoginResponse, error)
	Audit(context.Context, string, string, string, string, uint, string, string, string) error
	ListAudits(context.Context, int, int, string) ([]model.AuditLog, int64, error)
	AuditSummary(context.Context, time.Duration) (model.AuditSummary, error)
	EntityHistory(context.Context, string, uint, int) ([]model.AuditLog, error)
	RuntimeConfig() config.PublicConfig
	WithinTransaction(context.Context, func(context.Context) error) error
}

type securityService struct {
	repository repository.SecurityRepository
	config     config.Config
}

func NewSecurityService(repo repository.SecurityRepository, cfg config.Config) SecurityService {
	return &securityService{repository: repo, config: cfg}
}

func (s *securityService) Login(ctx context.Context, input dto.LoginRequest) (dto.LoginResponse, error) {
	user, err := s.repository.FindUserByUsername(ctx, input.Username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
		return dto.LoginResponse{}, ErrUnauthorized
	}
	if !user.Active {
		return dto.LoginResponse{}, ErrInactiveUser
	}
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub": fmt.Sprint(user.ID), "username": user.Username, "name": user.DisplayName,
		"role": user.Role, "iat": now.Unix(), "exp": now.Add(s.config.TokenTTL).Unix(),
		"iss": s.config.AppName,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return dto.LoginResponse{}, fmt.Errorf("sign access token: %w", err)
	}
	return dto.LoginResponse{
		Token: token, ExpiresIn: int64(s.config.TokenTTL.Seconds()), Username: user.Username,
		DisplayName: user.DisplayName, Role: user.Role,
	}, nil
}

func (s *securityService) Audit(ctx context.Context, actor, requestID, action, entityType string, entityID uint, before, after, detail string) error {
	if actor == "" {
		actor = "system"
	}
	if requestID == "" {
		requestID = "untracked"
	}
	if action == "" || entityType == "" {
		return fmt.Errorf("audit action and entity type are required")
	}
	return s.repository.AppendAudit(ctx, &model.AuditLog{
		Actor: actor, RequestID: requestID, Action: action, EntityType: entityType,
		EntityID: entityID, BeforeState: before, AfterState: after, Detail: detail,
		CreatedAt: time.Now().UTC(),
	})
}

func (s *securityService) ListAudits(ctx context.Context, page, pageSize int, search string) ([]model.AuditLog, int64, error) {
	return s.repository.ListAudits(ctx, page, pageSize, search)
}

func (s *securityService) AuditSummary(ctx context.Context, window time.Duration) (model.AuditSummary, error) {
	if window < time.Hour {
		window = 24 * time.Hour
	}
	if window > 90*24*time.Hour {
		window = 90 * 24 * time.Hour
	}
	return s.repository.SummarizeAudits(ctx, time.Now().UTC().Add(-window))
}

func (s *securityService) EntityHistory(ctx context.Context, entityType string, entityID uint, limit int) ([]model.AuditLog, error) {
	if entityID == 0 || entityType == "" {
		return nil, ErrInvalidInput
	}
	return s.repository.EntityHistory(ctx, entityType, entityID, limit)
}

func (s *securityService) RuntimeConfig() config.PublicConfig { return s.config.Public() }

func (s *securityService) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return s.repository.WithinTransaction(ctx, fn)
}
