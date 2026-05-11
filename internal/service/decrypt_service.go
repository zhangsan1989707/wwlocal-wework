package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"wwlocal-wework/internal/crypto"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type DecryptService struct {
	keyRepo *repository.KeyRepository
	cache   map[string]*crypto.RSADecryptor
}

func NewDecryptService(keyRepo *repository.KeyRepository) *DecryptService {
	return &DecryptService{
		keyRepo: keyRepo,
		cache:   make(map[string]*crypto.RSADecryptor),
	}
}

func (s *DecryptService) getDecryptor(version string) (*crypto.RSADecryptor, error) {
	if version == "" {
		return s.getActiveDecryptor()
	}

	if dec, ok := s.cache[version]; ok {
		return dec, nil
	}

	key, err := s.keyRepo.GetByVersion(version)
	if err != nil {
		return nil, fmt.Errorf("get key version %s failed: %w", version, err)
	}
	_ = key

	keyPath := s.keyRepo.GetKeyFilePath(version)
	dec, err := crypto.NewRSADecryptorFromFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("create RSA decryptor failed: %w", err)
	}

	s.cache[version] = dec
	return dec, nil
}

func (s *DecryptService) getActiveDecryptor() (*crypto.RSADecryptor, error) {
	activeKey, err := s.keyRepo.GetActive()
	if err != nil {
		return nil, fmt.Errorf("get active key failed: %w", err)
	}
	return s.getDecryptor(activeKey.Version)
}

func (s *DecryptService) decryptInternal(dec *crypto.RSADecryptor, item *model.WeWorkLogItem) (*model.LogEntry, error) {
	encKeyBytes, err := dec.Decrypt(item.EncKey)
	if err != nil {
		return nil, fmt.Errorf("RSA decrypt enc_key failed: %w", err)
	}

	aesDec, err := crypto.NewAESDecryptor(encKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("create AES decryptor failed: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(item.EncData)
	if err != nil {
		return nil, fmt.Errorf("decode enc_data failed: %w", err)
	}

	plaintext, err := aesDec.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("AES decrypt failed: %w", err)
	}

	// 截去尾部8字节
	if len(plaintext) > 8 {
		plaintext = plaintext[:len(plaintext)-8]
	}

	var parsedJSON map[string]interface{}
	if err := json.Unmarshal(plaintext, &parsedJSON); err != nil {
		return nil, fmt.Errorf("parse decrypted JSON failed: %w", err)
	}

	parsedJSONStr, _ := json.Marshal(parsedJSON)

	return &model.LogEntry{
		FeatureID:  item.FeatureID,
		LogTime:    item.LogTime,
		IDC:        item.IDC,
		EncData:    item.EncData,
		EncKey:     item.EncKey,
		RawJSON:    string(plaintext),
		ParsedJSON: string(parsedJSONStr),
	}, nil
}

func (s *DecryptService) Decrypt(item *model.WeWorkLogItem) (*model.LogEntry, error) {
	dec, err := s.getActiveDecryptor()
	if err != nil {
		return nil, err
	}
	return s.decryptInternal(dec, item)
}

func (s *DecryptService) DecryptWithKey(item *model.WeWorkLogItem, version string) (*model.LogEntry, error) {
	dec, err := s.getDecryptor(version)
	if err != nil {
		return nil, err
	}
	return s.decryptInternal(dec, item)
}
