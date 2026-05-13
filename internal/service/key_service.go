package service

import (
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
