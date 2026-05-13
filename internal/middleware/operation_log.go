package middleware

import (
	"log"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/service"
)

var actionMapping = map[string]string{
	"/api/v1/auth/login":          "login",
	"/api/v1/logs/sync":           "sync",
	"/api/v1/logs/sync/cancel":    "sync",
	"/api/v1/logs/sync/status":    "sync",
	"/api/v1/logs/query":          "query",
	"/api/v1/logs/features":       "query",
	"/api/v1/logs/time-range":     "query",
	"/api/v1/logs/field-paths":    "query",
	"/api/v1/keys":                "key_management",
	"/api/v1/keys/activate":       "key_management",
	"/api/v1/scheduler":           "scheduler",
	"/api/v1/scheduler/start":     "scheduler",
	"/api/v1/scheduler/stop":      "scheduler",
	"/api/v1/scheduler/sync":      "scheduler",
	"/api/v1/scheduler/interval":  "scheduler",
	"/api/v1/contacts":            "contacts",
	"/api/v1/contacts/sync":       "contacts",
	"/api/v1/contacts/sync/incremental": "contacts",
	"/api/v1/contacts/sync/cancel":      "contacts",
	"/api/v1/contacts/sync/status":      "contacts",
	"/api/v1/contacts/tree":             "contacts",
	"/api/v1/contacts/departments":      "contacts",
	"/api/v1/dashboard":                 "dashboard",
	"/api/v1/operation-logs":            "operation_log",
}

func resolveAction(path string) string {
	for prefix, action := range actionMapping {
		if strings.HasPrefix(path, prefix) {
			return action
		}
	}
	return "unknown"
}

func OperationLog(svc *service.OperationLogService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)

			if c.Request().URL.Path == "/health" {
				return err
			}

			username := ""
			if v, ok := c.Get("username").(string); ok {
				username = v
			}

			errorMsg := ""
			if err != nil {
				errorMsg = err.Error()
			}
			if v, ok := c.Get("op_error_msg").(string); ok {
				errorMsg = v
			}

			opLog := &model.OperationLog{
				Username:   username,
				Action:     resolveAction(c.Request().URL.Path),
				Method:     c.Request().Method,
				Path:       c.Request().URL.Path,
				StatusCode: c.Response().Status,
				ErrorMsg:   errorMsg,
				DurationMs: time.Since(start).Milliseconds(),
				IP:         c.RealIP(),
			}

			go func() {
				if saveErr := svc.Save(opLog); saveErr != nil {
					log.Printf("save operation log failed: %v", saveErr)
				}
			}()

			return err
		}
	}
}
