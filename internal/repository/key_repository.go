package repository

import (
	"os"
	"time"

	"gorm.io/gorm"
	"wwlocal-wework/internal/crypto"
	"wwlocal-wework/internal/model"
)

type KeyRepository struct {
	DB         *gorm.DB
	keysDir    string
	encryptKey string // hex 编码的 AES 密钥，为空则不加密
}

func NewKeyRepository(db *gorm.DB, keysDir, encryptKey string) *KeyRepository {
	return &KeyRepository{
		DB:         db,
		keysDir:    keysDir,
		encryptKey: encryptKey,
	}
}

func (r *KeyRepository) AutoMigrate() error {
	return r.DB.AutoMigrate(&model.RSAKeyVersion{})
}

func (r *KeyRepository) Create(key *model.RSAKeyVersion) error {
	return r.DB.Create(key).Error
}

func (r *KeyRepository) GetByVersion(version string) (*model.RSAKeyVersion, error) {
	var key model.RSAKeyVersion
	if err := r.DB.Where("version = ?", version).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *KeyRepository) GetActive() (*model.RSAKeyVersion, error) {
	var key model.RSAKeyVersion
	if err := r.DB.Where("is_active = ?", true).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *KeyRepository) GetAll() ([]model.RSAKeyVersion, error) {
	var keys []model.RSAKeyVersion
	if err := r.DB.Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *KeyRepository) SetActive(version string) error {
	tx := r.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 先取消所有版本的激活状态
	if err := tx.Model(&model.RSAKeyVersion{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 激活指定版本
	var key model.RSAKeyVersion
	if err := tx.Where("version = ?", version).First(&key).Error; err != nil {
		tx.Rollback()
		return err
	}

	key.IsActive = true
	now := time.Now()
	key.ActivatedAt = &now

	if err := tx.Save(&key).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (r *KeyRepository) SaveKeyToFile(version, privateKeyPEM string) error {
	dir := r.keysDir + "/" + version
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data := []byte(privateKeyPEM)
	if r.encryptKey != "" {
		encrypted, err := crypto.EncryptBytes(data, r.encryptKey)
		if err != nil {
			return err
		}
		data = encrypted
	}

	path := dir + "/rsa_private_key.pem"
	return os.WriteFile(path, data, 0600)
}

func (r *KeyRepository) ReadKeyFile(version string) ([]byte, error) {
	path := r.keysDir + "/" + version + "/rsa_private_key.pem"
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if r.encryptKey != "" {
		return crypto.DecryptBytes(data, r.encryptKey)
	}

	return data, nil
}

func (r *KeyRepository) GetKeyFilePath(version string) string {
	return r.keysDir + "/" + version + "/rsa_private_key.pem"
}