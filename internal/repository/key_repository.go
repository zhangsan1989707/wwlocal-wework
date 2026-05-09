package repository

import (
	"os"
	"time"

	"gorm.io/gorm"
	"wwlocal-wework/internal/model"
)

type KeyRepository struct {
	db         *gorm.DB
	keysDir    string
	keyCache   map[string]*model.RSAKeyVersion
}

func NewKeyRepository(db *gorm.DB, keysDir string) *KeyRepository {
	return &KeyRepository{
		db:      db,
		keysDir: keysDir,
		keyCache: make(map[string]*model.RSAKeyVersion),
	}
}

func (r *KeyRepository) AutoMigrate() error {
	return r.db.AutoMigrate(&model.RSAKeyVersion{})
}

func (r *KeyRepository) Create(key *model.RSAKeyVersion) error {
	return r.db.Create(key).Error
}

func (r *KeyRepository) GetByVersion(version string) (*model.RSAKeyVersion, error) {
	var key model.RSAKeyVersion
	if err := r.db.Where("version = ?", version).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *KeyRepository) GetActive() (*model.RSAKeyVersion, error) {
	var key model.RSAKeyVersion
	if err := r.db.Where("is_active = ?", true).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *KeyRepository) GetAll() ([]model.RSAKeyVersion, error) {
	var keys []model.RSAKeyVersion
	if err := r.db.Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *KeyRepository) SetActive(version string) error {
	tx := r.db.Begin()

	if err := tx.Model(&model.RSAKeyVersion{}).Update("is_active", false).Error; err != nil {
		tx.Rollback()
		return err
	}

	now := time.Now()
	if err := tx.Model(&model.RSAKeyVersion{}).Where("version = ?", version).Updates(map[string]interface{}{
		"is_active":    true,
		"activated_at": &now,
	}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (r *KeyRepository) SaveKeyToFile(version, privateKeyPEM string) error {
	dir := r.keysDir + "/" + version
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := dir + "/rsa_private_key.pem"
	return os.WriteFile(path, []byte(privateKeyPEM), 0600)
}

func (r *KeyRepository) GetKeyFilePath(version string) string {
	return r.keysDir + "/" + version + "/rsa_private_key.pem"
}