package model

import (
	"time"
)

type SyncState struct {
	FeatureID   int       `gorm:"primaryKey;column:feature_id" json:"feature_id"`
	LastLogTime int64     `gorm:"column:last_log_time;default:0" json:"last_log_time"`
	LastSyncAt  time.Time `gorm:"column:last_sync_at" json:"last_sync_at"`
	TotalSynced int64     `gorm:"column:total_synced;default:0" json:"total_synced"`
}

func (SyncState) TableName() string {
	return "sync_state"
}
