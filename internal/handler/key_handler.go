package handler

import (
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
		return response.Error(c, 400, "invalid request body")
	}

	if req.Version == "" || req.PrivateKeyPEM == "" {
		return response.Error(c, 400, "version and private_key_pem are required")
	}

	key, err := h.keySvc.AddKey(req.Version, req.PrivateKeyPEM)
	if err != nil {
		return response.Error(c, 500, "add key failed: "+err.Error())
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
