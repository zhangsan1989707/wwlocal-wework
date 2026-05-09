package response

import (
	"net/http"

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
	return c.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
	})
}

func ErrorWithStatus(c echo.Context, status int, code int, msg string) error {
	return c.JSON(status, Response{
		Code: code,
		Msg:  msg,
	})
}