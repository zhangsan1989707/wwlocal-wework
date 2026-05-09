package handler

import (
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
	"wwlocal-wework/pkg/response"

	"github.com/labstack/echo/v4"
)

type KeyHandler struct {
	keyRepo *repository.KeyRepository
}

func NewKeyHandler(keyRepo *repository.KeyRepository) *KeyHandler {
	return &KeyHandler{keyRepo: keyRepo}
}

func (h *KeyHandler) List(c echo.Context) error {
	keys, err := h.keyRepo.GetAll()
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

	if err := h.keyRepo.SaveKeyToFile(req.Version, req.PrivateKeyPEM); err != nil {
		return response.Error(c, 500, "save key file failed: "+err.Error())
	}

	key := &model.RSAKeyVersion{
		Version:        req.Version,
		PrivateKeyPath: h.keyRepo.GetKeyFilePath(req.Version),
	}

	if err := h.keyRepo.Create(key); err != nil {
		return response.Error(c, 500, "save key to database failed: "+err.Error())
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

	if err := h.keyRepo.SetActive(req.Version); err != nil {
		return response.Error(c, 500, "activate key failed: "+err.Error())
	}

	return response.Success(c, map[string]string{"message": "key activated", "version": req.Version})
}