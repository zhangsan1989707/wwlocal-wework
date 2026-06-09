package handler

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/config"
	"wwlocal-wework/internal/middleware"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type AuthHandler struct {
	cfg     *config.AuthConfig
	userSvc *service.UserService
	limiter *loginLimiter
}

func NewAuthHandler(cfg *config.AuthConfig, userSvc *service.UserService) *AuthHandler {
	return &AuthHandler{cfg: cfg, userSvc: userSvc, limiter: newLoginLimiter()}
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

	user, err := h.userSvc.Authenticate(req.Username, req.Password)
	if err != nil {
		h.limiter.RecordFailure(ip)
		return response.Error(c, 401, "invalid username or password")
	}
	h.limiter.RecordSuccess(ip)

	c.Set("username", user.Username)
	c.Set("user_id", user.ID)
	c.Set("role", user.Role)
	token, err := middleware.GenerateToken(user.ID, user.Username, user.Role, h.cfg.JWTSecret, 2*time.Hour)
	if err != nil {
		return response.Error(c, 500, "generate token failed")
	}

	refreshToken, err := middleware.GenerateRefreshToken(user.ID, user.Username, user.Role, h.cfg.JWTSecret, 7*24*time.Hour)
	if err != nil {
		return response.Error(c, 500, "generate refresh token failed")
	}

	return response.Success(c, map[string]interface{}{
		"token":         token,
		"refresh_token": refreshToken,
		"username":      user.Username,
		"role":          user.Role,
	})
}

func (h *AuthHandler) RefreshToken(c echo.Context) error {
	// 从请求体获取 refresh_token（也可从 Authorization header 获取）
	var req struct {
		Token string `json:"refresh_token"`
	}
	if err := c.Bind(&req); err != nil || req.Token == "" {
		return response.Error(c, 400, "refresh_token 不能为空")
	}

	// 验证 refresh token
	claims, err := middleware.ParseToken(req.Token, h.cfg.JWTSecret)
	if err != nil {
		return response.Error(c, 401, "refresh_token 无效或已过期")
	}

	// 检查 token 类型
	if claims.TokenType != "refresh" {
		return response.Error(c, 401, "无效的 token 类型")
	}

	// 生成新的 access token
	token, err := middleware.GenerateToken(claims.UserID, claims.Username, claims.Role, h.cfg.JWTSecret, 2*time.Hour)
	if err != nil {
		return response.Error(c, 500, "generate token failed")
	}

	return response.Success(c, map[string]interface{}{
		"token": token,
	})
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func validatePassword(pw string) error {
	var hasUpper, hasLower, hasDigit bool
	for _, ch := range pw {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		}
	}
	var errs []string
	if !hasUpper {
		errs = append(errs, "大写字母")
	}
	if !hasLower {
		errs = append(errs, "小写字母")
	}
	if !hasDigit {
		errs = append(errs, "数字")
	}
	if len(errs) > 0 {
		return fmt.Errorf("新密码必须包含: %s", strings.Join(errs, "、"))
	}
	return nil
}

func (h *AuthHandler) ChangePassword(c echo.Context) error {
	var req ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return response.Error(c, 400, "旧密码和新密码不能为空")
	}

	if len(req.NewPassword) < 8 {
		return response.Error(c, 400, "新密码长度不能少于 8 位")
	}

	if err := validatePassword(req.NewPassword); err != nil {
		return response.Error(c, 400, err.Error())
	}

	if req.OldPassword == req.NewPassword {
		return response.Error(c, 400, "新密码不能与旧密码相同")
	}

	userID := middleware.CurrentUserID(c)
	if userID <= 0 {
		return response.Error(c, 401, "invalid token")
	}
	if err := h.userSvc.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		return response.Error(c, 401, err.Error())
	}

	return response.Success(c, map[string]string{"message": "密码修改成功"})
}

// loginLimiter 简单的基于 IP 的登录限流：1 分钟内最多 5 次失败
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptEntry
	stopCh   chan struct{}
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

func (l *loginLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
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
