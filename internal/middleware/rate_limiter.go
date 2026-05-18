package middleware

import (
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// RateLimiter holds the state for the rate limiter
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	limit    rate.Limit
	burst    int
	ttl      time.Duration
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new RateLimiter instance
func NewRateLimiter(requestsPerMinute int, burst int) *RateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 100
	}
	if burst <= 0 {
		burst = 20
	}

	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    rate.Limit(requestsPerMinute) / rate.Limit(60),
		burst:    burst,
		ttl:      5 * time.Minute,
	}

	// Start cleanup goroutine
	go rl.cleanupVisitors()

	return rl
}

func (rl *RateLimiter) getVisitor(ip string) *rate.Limiter {
	rl.mu.RLock()
	v, exists := rl.visitors[ip]
	rl.mu.RUnlock()

	if exists {
		rl.mu.Lock()
		v.lastSeen = time.Now()
		rl.mu.Unlock()
		return v.limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check in case another goroutine added it
	if v, exists := rl.visitors[ip]; exists {
		v.lastSeen = time.Now()
		return v.limiter
	}

	limiter := rate.NewLimiter(rl.limit, rl.burst)
	rl.visitors[ip] = &visitor{
		limiter:  limiter,
		lastSeen: time.Now(),
	}
	return limiter
}

func (rl *RateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.ttl {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware returns an Echo middleware for rate limiting
func (rl *RateLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			limiter := rl.getVisitor(ip)

			if !limiter.Allow() {
				return echo.NewHTTPError(429, "Too many requests")
			}

			return next(c)
		}
	}
}
