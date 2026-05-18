package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/pkg/metrics"
)

func PrometheusMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			duration := time.Since(start)
			status := c.Response().Status
			method := c.Request().Method
			path := normalizePath(c.Path())

			metrics.RecordHttpRequest(method, path, status, duration)

			return err
		}
	}
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func RecordDBPoolStats(db interface {
	Stats() interface {
		GetMaxOpenConnections() int
		GetIdle() int
	}
}) {
	stats := db.Stats()
	metrics.SetDBPoolStats(stats.GetMaxOpenConnections(), stats.GetIdle())
}
