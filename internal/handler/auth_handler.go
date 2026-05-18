package handler

import (
	"log"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"wwlocal-wework/config"
	"wwlocal-wework/internal/middleware"
	"wwlocal-wework/internal/repository"
	"wwlocal-wework/pkg/response"
)

const passwordHashKey = "auth_password_hash"

type AuthHandler struct {
	cfg          *config.AuthConfig
	passwordHash []byte
	settingRepo  *repository.SettingRepository
	limiter      *loginLimiter
}

func NewAuthHandler(cfg *config.AuthConfig, settingRepo *repository.SettingRepository) *AuthHandler {
	var hash []byte

	// 优先从数据库加载已修改的密码哈希
	if settingRepo != nil {
		if stored, err := settingRepo.Get(passwordHashKey); err == nil && stored != "" {
			hash = []byte(stored)
			log.Println("loaded password hash from database")
		}
	}

	// 数据库没有则使用环境变量
	if hash == nil {
		hash, _ = bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
		log.Println("using password hash from environment config")
	}

	return &AuthHandler{cfg: cfg, passwordHash: hash, settingRepo: settingRepo, limiter: newLoginLimiter()}
}

func (h *AuthHandler) Stop() {
	h.limiter.stop()
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

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (h *AuthHandler) ChangePassword(c echo.Context) error {
	var req ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return response.Error(c, 400, "旧密码和新密码不能为空")
	}

	if len(req.NewPassword) < 6 {
		return response.Error(c, 400, "新密码长度不能少于 6 位")
	}

	if req.OldPassword == req.NewPassword {
		return response.Error(c, 400, "新密码不能与旧密码相同")
	}

	// 校验旧密码
	if bcrypt.CompareHashAndPassword(h.passwordHash, []byte(req.OldPassword)) != nil {
		return response.Error(c, 401, "旧密码不正确")
	}

	// 生成新密码哈希
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return response.Error(c, 500, "密码加密失败")
	}

	// 持久化到数据库
	if h.settingRepo != nil {
		if err := h.settingRepo.Set(passwordHashKey, string(newHash)); err != nil {
			return response.Error(c, 500, "保存密码失败")
		}
	}

	// 更新内存
	h.passwordHash = newHash
	log.Println("password changed successfully")

	return response.Success(c, map[string]string{"message": "密码修改成功"})
}

// loginLimiter 简单的基于 IP 的登录限流：1 分钟内最多 5 次失败
type loginLimiter struct {
	mu        sync.Mutex
	attempts  map[string]*attemptEntry
	stopCh    chan struct{}
}

type attemptEntry struct {
	count     int
	firstAt   time.Time
	blocked   bool
	blockedAt time.Time
}

func newLoginLimiter() *loginLimiter {
	l := &loginLimiter{attempts: make(map[string]*attemptEntry), stopCh: make(chan struct{})}
	go l.cleanup()
	return l
}

func (l *loginLimiter) stop() {
	close(l.stopCh)
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
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
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
}
