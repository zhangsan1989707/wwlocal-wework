package handler

import (
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"wwlocal-wework/config"
	"wwlocal-wework/internal/middleware"
	"wwlocal-wework/pkg/response"
)

type AuthHandler struct {
	cfg          *config.AuthConfig
	passwordHash []byte
	limiter      *loginLimiter
}

func NewAuthHandler(cfg *config.AuthConfig) *AuthHandler {
	hash, _ := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
	return &AuthHandler{cfg: cfg, passwordHash: hash, limiter: newLoginLimiter()}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c echo.Context) error {
	ip := c.RealIP()
	if !h.limiter.Allow(ip) {
		return response.Error(c, 429, "登录尝试过于频繁，请稍后再试")
	}

	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	if req.Username != h.cfg.Username || bcrypt.CompareHashAndPassword(h.passwordHash, []byte(req.Password)) != nil {
		h.limiter.RecordFailure(ip)
		return response.Error(c, 401, "invalid username or password")
	}

	c.Set("username", req.Username)
	token, err := middleware.GenerateToken(req.Username, h.cfg.JWTSecret, 24*time.Hour)
	if err != nil {
		return response.Error(c, 500, "generate token failed")
	}

	return response.Success(c, map[string]interface{}{
		"token":    token,
		"username": req.Username,
	})
}

// loginLimiter 简单的基于 IP 的登录限流：1 分钟内最多 5 次失败
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptEntry
}

type attemptEntry struct {
	count    int
	firstAt  time.Time
	blocked  bool
	blockedAt time.Time
}

func newLoginLimiter() *loginLimiter {
	l := &loginLimiter{attempts: make(map[string]*attemptEntry)}
	go l.cleanup()
	return l
}

func (l *loginLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.attempts[ip]
	if !ok {
		return true
	}
	if e.blocked && time.Since(e.blockedAt) < time.Minute {
		return false
	}
	if e.blocked && time.Since(e.blockedAt) >= time.Minute {
		delete(l.attempts, ip)
		return true
	}
	return true
}

func (l *loginLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.attempts[ip]
	if !ok || time.Since(e.firstAt) >= time.Minute {
		l.attempts[ip] = &attemptEntry{count: 1, firstAt: time.Now()}
		return
	}
	e.count++
	if e.count >= 5 {
		e.blocked = true
		e.blockedAt = time.Now()
	}
}

func (l *loginLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		l.mu.Lock()
		for ip, e := range l.attempts {
			if e.blocked && time.Since(e.blockedAt) >= time.Minute {
				delete(l.attempts, ip)
			} else if !e.blocked && time.Since(e.firstAt) >= time.Minute {
				delete(l.attempts, ip)
			}
		}
		l.mu.Unlock()
	}
}
