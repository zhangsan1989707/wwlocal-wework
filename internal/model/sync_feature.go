package model

import "time"

type SyncFeature struct {
	FeatureID int    `gorm:"primaryKey;column:feature_id" json:"feature_id"`
	Name      string `gorm:"type:varchar(128)" json:"name"`
	Enabled   bool   `gorm:"default:true" json:"enabled"`
}

func (SyncFeature) TableName() string { return "sync_features" }

type SyncFeatureWithState struct {
	SyncFeature
	LastSyncAt  *time.Time `json:"last_sync_at"`
	TotalSynced int64      `json:"total_synced"`
	LastLogTime int64      `json:"last_log_time"`
}
