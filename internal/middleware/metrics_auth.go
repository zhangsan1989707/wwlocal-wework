package middleware

import (
	"net"
	"net/http"

	"github.com/labstack/echo/v4"
)

// MetricsAuth 返回一个 IP 白名单中间件，仅允许指定 IP 访问 /metrics 端点
func MetricsAuth(allowedIPs []string) echo.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(allowedIPs))
	for _, ip := range allowedIPs {
		allowed[ip] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			// 去掉端口号
			if host, _, err := net.SplitHostPort(ip); err == nil {
				ip = host
			}
			if _, ok := allowed[ip]; !ok {
				return c.JSON(http.StatusForbidden, map[string]interface{}{
					"code": 403,
					"msg":  "forbidden",
				})
			}
			return next(c)
		}
	}
}
