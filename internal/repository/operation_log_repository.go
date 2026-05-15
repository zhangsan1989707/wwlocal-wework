package repository

import (
	"wwlocal-wework/internal/model"

	"gorm.io/gorm"
)

type OperationLogRepository struct {
	DB *gorm.DB
}

func NewOperationLogRepository(db *gorm.DB) *OperationLogRepository {
	return &OperationLogRepository{DB: db}
}

func (r *OperationLogRepository) AutoMigrate() error {
	return r.DB.AutoMigrate(&model.OperationLog{})
}

func (r *OperationLogRepository) Create(entry *model.OperationLog) error {
	return r.DB.Create(entry).Error
}

func (r *OperationLogRepository) List(page, pageSize int, action string, statusCode int) ([]model.OperationLog, int64, error) {
	var logs []model.OperationLog
	var total int64
	query := r.DB.Model(&model.OperationLog{}).Order("created_at DESC")
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if statusCode > 0 {
		query = query.Where("status_code = ?", statusCode)
	}
	query.Count(&total)
	offset := (page - 1) * pageSize
	err := query.Limit(pageSize).Offset(offset).Find(&logs).Error
	return logs, total, err
}

func (r *OperationLogRepository) GetDistinctActions() ([]string, error) {
	var actions []string
	err := r.DB.Model(&model.OperationLog{}).Distinct("action").Order("action").Pluck("action", &actions).Error
	return actions, err
}
