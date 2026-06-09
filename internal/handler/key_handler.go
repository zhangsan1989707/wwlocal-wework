package handler

import (
	"errors"

	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"

	"github.com/labstack/echo/v4"
)

type KeyHandler struct {
	keySvc *service.KeyService
}

func NewKeyHandler(keySvc *service.KeyService) *KeyHandler {
	return &KeyHandler{keySvc: keySvc}
}

func (h *KeyHandler) List(c echo.Context) error {
	keys, err := h.keySvc.ListKeys()
	if err != nil {
		return response.Error(c, 500, err.Error())
	}
	return response.Success(c, keys)
}

func (h *KeyHandler) Add(c echo.Context) error {
	var req model.AddKeyRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "请求体格式无效")
	}

	if req.Version == "" || req.PrivateKeyPEM == "" {
		return response.Error(c, 400, "请填写版本号和私钥内容")
	}

	key, err := h.keySvc.AddKey(req.Version, req.PrivateKeyPEM)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrKeyVersionExists):
			return response.Error(c, 409, err.Error())
		case errors.Is(err, service.ErrInvalidKeyVersion), errors.Is(err, service.ErrInvalidPrivateKey):
			return response.Error(c, 400, err.Error())
		default:
			return response.Error(c, 500, "添加密钥失败: "+err.Error())
		}
	}

	return response.Success(c, key)
}

func (h *KeyHandler) Activate(c echo.Context) error {
	var req model.ActivateKeyRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	if req.Version == "" {
		return response.Error(c, 400, "version is required")
	}

	if err := h.keySvc.ActivateKey(req.Version); err != nil {
		return response.Error(c, 500, "activate key failed: "+err.Error())
	}

	return response.Success(c, map[string]string{"message": "key activated", "version": req.Version})
}

func (h *KeyHandler) Test(c echo.Context) error {
	version := c.QueryParam("version")
	if version == "" {
		return response.Error(c, 400, "version is required")
	}

	result, err := h.keySvc.TestKey(version)
	if err != nil {
		return response.Error(c, 500, err.Error())
	}

	return response.Success(c, result)
}
