package model

import (
	"time"
)

type RSAKeyVersion struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Version       string     `gorm:"column:version;type:varchar(32);uniqueIndex" json:"version"`
	PrivateKeyPath string    `gorm:"column:private_key_path;type:varchar(255)" json:"private_key_path"`
	IsActive      bool       `gorm:"column:is_active;default:false" json:"is_active"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	ActivatedAt   *time.Time `gorm:"column:activated_at" json:"activated_at,omitempty"`
}

func (RSAKeyVersion) TableName() string {
	return "rsa_key_versions"
}

type AddKeyRequest struct {
	Version        string `json:"version" validate:"required"`
	PrivateKeyPEM  string `json:"private_key_pem" validate:"required"`
}

type ActivateKeyRequest struct {
	Version string `json:"version" validate:"required"`
}