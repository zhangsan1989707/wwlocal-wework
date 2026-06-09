package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"wwlocal-wework/internal/model"
)

type SettingRepository struct {
	DB *gorm.DB
}

func NewSettingRepository(db *gorm.DB) *SettingRepository {
	return &SettingRepository{DB: db}
}

func (r *SettingRepository) AutoMigrate() error {
	return r.DB.AutoMigrate(&model.Setting{})
}

func (r *SettingRepository) Get(key string) (string, error) {
	var s model.Setting
	if err := r.DB.Where("`key` = ?", key).First(&s).Error; err != nil {
		return "", err
	}
	return s.Value, nil
}

func (r *SettingRepository) Set(key, value string) error {
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&model.Setting{Key: key, Value: value}).Error
}
