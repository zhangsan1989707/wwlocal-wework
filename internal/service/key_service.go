package service

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
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
