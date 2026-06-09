package service

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"

	"gorm.io/gorm"
)

var (
	ErrKeyVersionExists  = errors.New("密钥版本已存在")
	ErrInvalidKeyVersion = errors.New("密钥版本只能包含字母、数字、点、下划线和短横线")
	ErrInvalidPrivateKey = errors.New("密钥内容不是有效的 RSA 私钥")
)

type KeyService struct {
	keyRepo *repository.KeyRepository
}

func NewKeyService(keyRepo *repository.KeyRepository) *KeyService {
	return &KeyService{keyRepo: keyRepo}
}

func (s *KeyService) ListKeys() ([]model.RSAKeyVersion, error) {
	return s.keyRepo.GetAll()
}

func (s *KeyService) AddKey(version, privateKeyPEM string) (*model.RSAKeyVersion, error) {
	version = strings.TrimSpace(version)
	privateKeyPEM = strings.TrimSpace(privateKeyPEM)

	if !validKeyVersion(version) {
		return nil, ErrInvalidKeyVersion
	}
	if err := validateRSAPrivateKeyPEM(privateKeyPEM); err != nil {
		return nil, err
	}
	if _, err := s.keyRepo.GetByVersion(version); err == nil {
		return nil, ErrKeyVersionExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if err := s.keyRepo.SaveKeyToFile(version, privateKeyPEM); err != nil {
		return nil, err
	}

	key := &model.RSAKeyVersion{
		Version:        version,
		PrivateKeyPath: s.keyRepo.GetKeyFilePath(version),
	}

	if err := s.keyRepo.Create(key); err != nil {
		return nil, err
	}

	return key, nil
}

func validKeyVersion(version string) bool {
	if version == "" || version == "." || version == ".." || len(version) > 32 {
		return false
	}
	for _, r := range version {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validateRSAPrivateKeyPEM(privateKeyPEM string) error {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return ErrInvalidPrivateKey
	}

	if priv, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		if priv.N.BitLen() > 0 {
			return nil
		}
		return ErrInvalidPrivateKey
	}

	keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return ErrInvalidPrivateKey
	}
	if _, ok := keyInterface.(*rsa.PrivateKey); !ok {
		return ErrInvalidPrivateKey
	}
	return nil
}

func (s *KeyService) ActivateKey(version string) error {
	return s.keyRepo.SetActive(version)
}

func (s *KeyService) TestKey(version string) (map[string]interface{}, error) {
	data, err := s.keyRepo.ReadKeyFile(version)
	if err != nil {
		return nil, fmt.Errorf("读取密钥文件失败: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("密钥格式无效：不是 PEM 格式")
	}

	var keySize int
	var keyType string

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		keySize = priv.N.BitLen()
		keyType = "PKCS1"
	} else {
		keyInterface, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("解析私钥失败: %v / %v", err, err2)
		}
		rsaKey, ok := keyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("不是 RSA 私钥")
		}
		keySize = rsaKey.N.BitLen()
		keyType = "PKCS8"
	}

	return map[string]interface{}{
		"valid":    true,
		"key_size": keySize,
		"key_type": keyType,
	}, nil
}
