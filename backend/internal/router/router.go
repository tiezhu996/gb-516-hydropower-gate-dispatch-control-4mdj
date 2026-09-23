package router

import (
	"log/slog"
	"net/http"

	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/config"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/handler"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/middleware"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/repository"
	"github.com/blueship581/hydropower-gate-dispatch-control/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client, logger *slog.Logger) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(middleware.RequestContext(logger))
	if len(cfg.TrustedProxies) > 0 {
		_ = engine.SetTrustedProxies(cfg.TrustedProxies)
	} else {
		_ = engine.SetTrustedProxies(nil)
	}

	securityRepository := repository.NewSecurityRepository(db)
	securityService := service.NewSecurityService(securityRepository, cfg)
	reservoirRepository := repository.NewReservoirRepository(db)
	gateUnitRepository := repository.NewGateUnitRepository(db)
	operationDirectiveRepository := repository.NewOperationDirectiveRepository(db)
	executionConfirmationRepository := repository.NewExecutionConfirmationRepository(db)
	reservoirService := service.NewReservoirService(reservoirRepository, securityService)
	gateUnitService := service.NewGateUnitService(gateUnitRepository, reservoirRepository, securityService)
	operationDirectiveService := service.NewOperationDirectiveService(operationDirectiveRepository, gateUnitRepository, securityService)
	executionConfirmationService := service.NewExecutionConfirmationService(executionConfirmationRepository, operationDirectiveRepository, gateUnitRepository, securityService)
	reservoirHandler := handler.NewReservoirHandler(reservoirService)
	gateUnitHandler := handler.NewGateUnitHandler(gateUnitService)
	operationDirectiveHandler := handler.NewOperationDirectiveHandler(operationDirectiveService)
	executionConfirmationHandler := handler.NewExecutionConfirmationHandler(executionConfirmationService)
	systemHandler := handler.NewSystemHandler(securityService, reservoirService, gateUnitService, operationDirectiveService, executionConfirmationService, db, redisClient)

	engine.GET("/healthz", systemHandler.Health)
	loginLimiter := middleware.NewLimiter(redisClient, cfg.LoginRequestLimit)
	engine.POST("/api/auth/login", loginLimiter.Middleware(), systemHandler.Login)

	limiter := middleware.NewLimiter(redisClient, cfg.RequestLimit)
	api := engine.Group("/api")
	api.Use(limiter.Middleware(), middleware.Authenticate(cfg, securityRepository))
	api.GET("/overview", systemHandler.Overview)
	api.GET("/session", systemHandler.Session)
	api.GET("/runtime", systemHandler.Runtime)
	api.GET("/audits", middleware.RequireRoles("reviewer", "admin"), systemHandler.Audits)
	api.GET("/audit-summary", middleware.RequireRoles("reviewer", "admin"), systemHandler.AuditSummary)
	api.GET("/audits/:entityType/:id", middleware.RequireRoles("reviewer", "admin"), systemHandler.EntityHistory)
	reservoirHandler.Register(api)
	gateUnitHandler.Register(api)
	operationDirectiveHandler.Register(api)
	executionConfirmationHandler.Register(api)

	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "route_not_found"})
	})
	return engine
}
