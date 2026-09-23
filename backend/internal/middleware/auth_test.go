package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/config"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type staticUserResolver struct {
	user model.User
	err  error
}

func (r staticUserResolver) FindUserByUsername(context.Context, string) (model.User, error) {
	return r.user, r.err
}

func authToken(t *testing.T, cfg config.Config, subject, role string) string {
	t.Helper()
	now := time.Now().UTC()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": subject, "username": "operator", "name": "Token Name", "role": role,
		"iss": cfg.AppName, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func TestAuthenticateUsesCurrentDatabaseRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{AppName: "test", JWTSecret: "test-secret-that-is-long-enough"}
	resolver := staticUserResolver{user: model.User{ID: 7, Username: "operator", DisplayName: "Current Name", Role: model.RoleViewer, Active: true}}
	engine := gin.New()
	engine.GET("/admin", Authenticate(cfg, resolver), RequireRoles(model.RoleAdmin), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.Header.Set("Authorization", "Bearer "+authToken(t, cfg, "7", model.RoleAdmin))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("stale admin claim must not override current viewer role: status=%d", response.Code)
	}
}

func TestAuthenticateRejectsInactiveAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{AppName: "test", JWTSecret: "test-secret-that-is-long-enough"}
	resolver := staticUserResolver{user: model.User{ID: 7, Username: "operator", Role: model.RoleOperator, Active: false}}
	engine := gin.New()
	engine.GET("/private", Authenticate(cfg, resolver), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.Header.Set("Authorization", "Bearer "+authToken(t, cfg, "7", model.RoleOperator))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("inactive account must be rejected: status=%d", response.Code)
	}
}
