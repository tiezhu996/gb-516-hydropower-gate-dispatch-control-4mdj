package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type bucket struct {
	window time.Time
	count  int
}

type Limiter struct {
	redis *redis.Client
	limit int
	mu    sync.Mutex
	local map[string]bucket
}

type RateLimitStatus struct {
	Limit     int       `json:"limit"`
	Tracked   int       `json:"trackedKeys"`
	WindowEnd time.Time `json:"windowEnd"`
	Backend   string    `json:"backend"`
}

func NewLimiter(client *redis.Client, limit int) *Limiter {
	return &Limiter{redis: client, limit: limit, local: make(map[string]bucket)}
}

func (l *Limiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP() + ":" + c.FullPath()
		count, err := l.increment(c.Request.Context(), key)
		if err != nil {
			count = l.incrementLocal(key)
		}
		c.Header("X-RateLimit-Limit", fmt.Sprint(l.limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprint(max(0, l.limit-count)))
		resetAt := time.Now().UTC().Truncate(time.Minute).Add(time.Minute)
		c.Header("X-RateLimit-Reset", fmt.Sprint(resetAt.Unix()))
		if count > l.limit {
			c.Header("Retry-After", fmt.Sprint(max(1, int(time.Until(resetAt).Seconds()))))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited"})
			return
		}
		c.Next()
	}
}

func (l *Limiter) increment(ctx context.Context, key string) (int, error) {
	if l.redis == nil {
		return 0, fmt.Errorf("redis disabled")
	}
	redisKey := "rate:" + time.Now().UTC().Format("200601021504") + ":" + key
	pipeline := l.redis.TxPipeline()
	counter := pipeline.Incr(ctx, redisKey)
	pipeline.Expire(ctx, redisKey, 2*time.Minute)
	if _, err := pipeline.Exec(ctx); err != nil {
		return 0, err
	}
	return int(counter.Val()), nil
}

func (l *Limiter) incrementLocal(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().UTC().Truncate(time.Minute)
	l.cleanupLocal(now)
	value := l.local[key]
	if !value.window.Equal(now) {
		value = bucket{window: now}
	}
	value.count++
	l.local[key] = value
	return value.count
}

func (l *Limiter) cleanupLocal(currentWindow time.Time) {
	if len(l.local) < 512 {
		return
	}
	for key, value := range l.local {
		if value.window.Before(currentWindow) {
			delete(l.local, key)
		}
	}
}

func (l *Limiter) Status() RateLimitStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	backend := "memory"
	if l.redis != nil {
		backend = "redis"
	}
	windowEnd := time.Now().UTC().Truncate(time.Minute).Add(time.Minute)
	return RateLimitStatus{Limit: l.limit, Tracked: len(l.local), WindowEnd: windowEnd, Backend: backend}
}
