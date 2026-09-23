package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestContext(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := normalizeRequestID(c.GetHeader("X-Request-ID"))
		c.Set("requestID", requestID)
		c.Header("X-Request-ID", requestID)
		applySecurityHeaders(c)
		started := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", "request_id", requestID, "panic", recovered, "stack", string(debug.Stack()))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "requestId": requestID})
			}
			duration := time.Since(started)
			c.Header("Server-Timing", "app;dur="+formatMilliseconds(duration))
			logger.Info("request", "request_id", requestID, "method", c.Request.Method,
				"path", c.Request.URL.Path, "status", c.Writer.Status(), "bytes", c.Writer.Size(),
				"client_ip", c.ClientIP(), "duration", duration)
		}()
		c.Next()
	}
}

func normalizeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return uuid.NewString()
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("._:-", character)) {
			return uuid.NewString()
		}
	}
	return value
}

func applySecurityHeaders(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	c.Header("Cross-Origin-Resource-Policy", "same-origin")
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.Header("Cache-Control", "no-store")
	}
}

func formatMilliseconds(duration time.Duration) string {
	return strconv.FormatFloat(float64(duration.Microseconds())/1000, 'f', 3, 64)
}
