package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/middleware"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type UserHandler struct {
	userSvc *service.UserService
}

func NewUserHandler(userSvc *service.UserService) *UserHandler {
	return &UserHandler{userSvc: userSvc}
}

func requireSuperAdmin(c echo.Context) error {
	if middleware.CurrentRole(c) != model.RoleSuperAdmin {
		return response.Forbidden(c, "仅超级管理员可操作")
	}
	return nil
}

func (h *UserHandler) List(c echo.Context) error {
	if err := requireSuperAdmin(c); err != nil {
		return err
	}
	users, err := h.userSvc.ListUsers()
	if err != nil {
		return response.Error(c, 500, "查询用户失败")
	}
	return response.Success(c, users)
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Enabled  *bool  `json:"enabled"`
	DeptIDs  []int  `json:"dept_ids"`
}

func (h *UserHandler) Create(c echo.Context) error {
	if err := requireSuperAdmin(c); err != nil {
		return err
	}
	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}
	if req.Username == "" || req.Password == "" {
		return response.Error(c, 400, "用户名和密码不能为空")
	}
	if len(req.Password) < 8 {
		return response.Error(c, 400, "密码长度不能少于 8 位")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	user, err := h.userSvc.CreateUser(req.Username, req.Password, req.Role, enabled, req.DeptIDs)
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	return response.Success(c, user)
}

type UpdateUserRequest struct {
	Role    string `json:"role"`
	Enabled bool   `json:"enabled"`
	DeptIDs []int  `json:"dept_ids"`
}

func (h *UserHandler) Update(c echo.Context) error {
	if err := requireSuperAdmin(c); err != nil {
		return err
	}
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		return response.Error(c, 400, "invalid user id")
	}
	var req UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}
	user, err := h.userSvc.UpdateUser(userID, req.Role, req.Enabled, req.DeptIDs)
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	return response.Success(c, user)
}

type ResetPasswordRequest struct {
	Password string `json:"password"`
}

func (h *UserHandler) ResetPassword(c echo.Context) error {
	if err := requireSuperAdmin(c); err != nil {
		return err
	}
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		return response.Error(c, 400, "invalid user id")
	}
	var req ResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}
	if len(req.Password) < 8 {
		return response.Error(c, 400, "密码长度不能少于 8 位")
	}
	if err := h.userSvc.ResetPassword(userID, req.Password); err != nil {
		return response.Error(c, 400, err.Error())
	}
	return response.Success(c, map[string]string{"message": "密码已重置"})
}
