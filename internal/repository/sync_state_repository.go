package repository

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"wwlocal-wework/internal/model"
)

type SyncStateRepository struct {
	db *gorm.DB
}

func NewSyncStateRepository(db *gorm.DB) *SyncStateRepository {
	return &SyncStateRepository{db: db}
}

func (r *SyncStateRepository) AutoMigrate() error {
	return r.db.AutoMigrate(&model.SyncState{})
}

func (r *SyncStateRepository) GetByFeatureID(featureID int) (*model.SyncState, error) {
	var state model.SyncState
	if err := r.db.Where("feature_id = ?", featureID).First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *SyncStateRepository) GetLastLogTime(featureID int) int64 {
	state, err := r.GetByFeatureID(featureID)
	if err != nil {
		return 0
	}
	return state.LastLogTime
}

func (r *SyncStateRepository) UpdateState(featureID int, lastLogTime int64, count int) error {
	now := time.Now()
	state := model.SyncState{
		FeatureID:   featureID,
		LastLogTime: lastLogTime,
		LastSyncAt:  now,
		TotalSynced: int64(count),
	}

	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "feature_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_log_time": gorm.Expr("GREATEST(last_log_time, ?)", lastLogTime),
			"last_sync_at":  now,
			"total_synced":  gorm.Expr("total_synced + ?", count),
		}),
	}).Create(&state).Error
}

func (r *SyncStateRepository) GetAll() ([]model.SyncState, error) {
	var states []model.SyncState
	if err := r.db.Find(&states).Error; err != nil {
		return nil, err
	}
	return states, nil
}
