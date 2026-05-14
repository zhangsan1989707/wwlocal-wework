package repository

import (
	"wwlocal-wework/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SyncFeatureRepository struct {
	db *gorm.DB
}

func NewSyncFeatureRepository(db *gorm.DB) *SyncFeatureRepository {
	return &SyncFeatureRepository{db: db}
}

func (r *SyncFeatureRepository) AutoMigrate() error {
	return r.db.AutoMigrate(&model.SyncFeature{})
}

func (r *SyncFeatureRepository) GetAll() ([]model.SyncFeatureWithState, error) {
	var results []model.SyncFeatureWithState
	err := r.db.Table("sync_features sf").
		Select("sf.feature_id, sf.name, sf.enabled, ss.last_sync_at, COALESCE(ss.total_synced, 0) as total_synced, COALESCE(ss.last_log_time, 0) as last_log_time").
		Joins("LEFT JOIN sync_state ss ON ss.feature_id = sf.feature_id").
		Order("sf.feature_id").
		Scan(&results).Error
	return results, err
}

func (r *SyncFeatureRepository) GetEnabledIDs() ([]int, error) {
	var ids []int
	err := r.db.Model(&model.SyncFeature{}).Where("enabled = ?", true).Pluck("feature_id", &ids).Error
	return ids, err
}

func (r *SyncFeatureRepository) BatchUpsert(features []model.SyncFeature) error {
	if len(features) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "feature_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name"}),
	}).Create(&features).Error
}

func (r *SyncFeatureRepository) SetEnabled(featureID int, enabled bool) error {
	return r.db.Model(&model.SyncFeature{}).Where("feature_id = ?", featureID).Update("enabled", enabled).Error
}

func (r *SyncFeatureRepository) BatchSetEnabled(updates map[int]bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for fid, enabled := range updates {
			if err := tx.Model(&model.SyncFeature{}).Where("feature_id = ?", fid).Update("enabled", enabled).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
