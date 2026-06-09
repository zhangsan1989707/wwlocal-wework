package repository

import (
	"fmt"

	"wwlocal-wework/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SyncFeatureRepository struct {
	DB *gorm.DB
}

func NewSyncFeatureRepository(db *gorm.DB) *SyncFeatureRepository {
	return &SyncFeatureRepository{DB: db}
}

func (r *SyncFeatureRepository) AutoMigrate() error {
	return r.DB.AutoMigrate(&model.SyncFeature{})
}

func (r *SyncFeatureRepository) GetAll() ([]model.SyncFeatureWithState, error) {
	var results []model.SyncFeatureWithState
	err := r.DB.Table("sync_features sf").
		Select("sf.feature_id, sf.name, sf.enabled, ss.last_sync_at, COALESCE(ss.total_synced, 0) as total_synced, COALESCE(ss.last_log_time, 0) as last_log_time").
		Joins("LEFT JOIN sync_state ss ON ss.feature_id = sf.feature_id").
		Order("sf.feature_id").
		Scan(&results).Error
	return results, err
}

func (r *SyncFeatureRepository) GetEnabledIDs() ([]int, error) {
	var ids []int
	err := r.DB.Model(&model.SyncFeature{}).Where("enabled = ?", true).Pluck("feature_id", &ids).Error
	return ids, err
}

func (r *SyncFeatureRepository) BatchUpsert(features []model.SyncFeature) error {
	if len(features) == 0 {
		return nil
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "feature_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name"}),
	}).Create(&features).Error
}

func (r *SyncFeatureRepository) SetEnabled(featureID int, enabled bool) error {
	return r.DB.Model(&model.SyncFeature{}).Where("feature_id = ?", featureID).Update("enabled", enabled).Error
}

func (r *SyncFeatureRepository) BatchSetEnabled(updates map[int]bool) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		for fid, enabled := range updates {
			result := tx.Model(&model.SyncFeature{}).Where("feature_id = ?", fid).Update("enabled", enabled)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("sync feature %d not found", fid)
			}
		}
		return nil
	})
}
