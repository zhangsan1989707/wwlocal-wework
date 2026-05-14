package repository

import (
	"time"

	"gorm.io/gorm"
	"wwlocal-wework/internal/model"
)

type SyncHistoryRepository struct {
	db *gorm.DB
}

func NewSyncHistoryRepository(db *gorm.DB) *SyncHistoryRepository {
	return &SyncHistoryRepository{db: db}
}

func (r *SyncHistoryRepository) AutoMigrate() error {
	return r.db.AutoMigrate(&model.SyncHistory{})
}

func (r *SyncHistoryRepository) Create(h *model.SyncHistory) error {
	return r.db.Create(h).Error
}

func (r *SyncHistoryRepository) List(syncType string, page, pageSize int) ([]model.SyncHistory, int64, error) {
	query := r.db.Model(&model.SyncHistory{})
	if syncType != "" {
		query = query.Where("sync_type = ?", syncType)
	}

	var total int64
	query.Count(&total)

	var items []model.SyncHistory
	offset := (page - 1) * pageSize
	if err := query.Order("start_time DESC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *SyncHistoryRepository) GetLatest(syncType string) (*model.SyncHistory, error) {
	var h model.SyncHistory
	err := r.db.Where("sync_type = ?", syncType).Order("start_time DESC").First(&h).Error
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *SyncHistoryRepository) GetStats(syncType string, since time.Time) (totalRuns int, totalRecords int, totalFailed int, err error) {
	result := r.db.Model(&model.SyncHistory{}).
		Where("sync_type = ? AND start_time >= ?", syncType, since).
		Select("COUNT(*), COALESCE(SUM(succeeded),0), COALESCE(SUM(failed),0)").
		Row()
	err = result.Scan(&totalRuns, &totalRecords, &totalFailed)
	return
}
