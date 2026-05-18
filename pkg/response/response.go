package response

import (
	"log"
	"net/http"
	"runtime"
	"strings"

	"github.com/labstack/echo/v4"
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func Success(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  "success",
		Data: data,
	})
}

func Error(c echo.Context, code int, msg string) error {
	c.Set("op_error_msg", msg)
	httpStatus := codeToHTTPStatus(code)
	return c.JSON(httpStatus, Response{
		Code: code,
		Msg:  msg,
	})
}

func ErrorWithStatus(c echo.Context, status int, code int, msg string) error {
	c.Set("op_error_msg", msg)
	return c.JSON(status, Response{
		Code: code,
		Msg:  msg,
	})
}

func ServerError(c echo.Context, contextMsg string) error {
	c.Set("op_error_msg", contextMsg)
	log.Printf("[ERROR] %s\n%s", contextMsg, getCallerStack())
	return c.JSON(http.StatusInternalServerError, Response{
		Code: 500,
		Msg:  "服务器内部错误，请稍后再试",
	})
}

func BadRequest(c echo.Context, msg string) error {
	return Error(c, http.StatusBadRequest, msg)
}

func Unauthorized(c echo.Context, msg string) error {
	return Error(c, http.StatusUnauthorized, msg)
}

func Forbidden(c echo.Context, msg string) error {
	return Error(c, http.StatusForbidden, msg)
}

func NotFound(c echo.Context, msg string) error {
	return Error(c, http.StatusNotFound, msg)
}

func codeToHTTPStatus(code int) int {
	switch {
	case code >= 400 && code < 500:
		return code
	case code >= 500:
		return code
	default:
		return http.StatusOK
	}
}

func getCallerStack() string {
	var pcs [32]uintptr
	n := runtime.Callers(3, pcs[:])
	var sb strings.Builder
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !contains(frame.Function, []string{"response.go", "handler.go"}) {
			sb.WriteString(frame.File)
			sb.WriteString(":")
			sb.WriteString(formatFileLine(frame.Line))
			sb.WriteString(" ")
			sb.WriteString(frame.Function)
			sb.WriteString("\n")
		}
		if !more {
			break
		}
	}
	return sb.String()
}

func contains(s string, subs []string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) && s[len(s)-len(sub):] == sub {
			return true
		}
	}
	return false
}

func formatFileLine(line int) string {
	return string(rune('0'+line%10)) + string(rune('0'+(line/10)%10))
}
