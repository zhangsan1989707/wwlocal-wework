package model

import (
	"time"
)

type LogEntry struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	FeatureID  int       `gorm:"column:feature_id;not null;index:idx_feature_logtime" json:"feature_id"`
	LogTime    int64     `gorm:"column:log_time;not null;index:idx_feature_logtime" json:"log_time"`
	IDC        string    `gorm:"column:idc;type:varchar(32)" json:"idc"`
	EncData    string    `gorm:"column:enc_data;type:text" json:"enc_data"`
	EncKey     string    `gorm:"column:enc_key;type:text" json:"enc_key"`
	RawJSON    string    `gorm:"column:raw_json;type:text" json:"raw_json"`
	ParsedJSON string    `gorm:"column:parsed_json;type:json" json:"parsed_json"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (LogEntry) TableName() string {
	return "log_entries"
}

type LogEntryQuery struct {
	FeatureIDs []int
	StartTime  int64
	EndTime    int64
	Conditions map[string]interface{}
	Page       int
	PageSize   int
}

type LogSyncRecord struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	FeatureID    int       `gorm:"column:feature_id;not null" json:"feature_id"`
	SyncTime     time.Time `gorm:"column:sync_time;autoCreateTime" json:"sync_time"`
	StartTime    int64     `gorm:"column:start_time" json:"start_time"`
	EndTime      int64     `gorm:"column:end_time" json:"end_time"`
	StartIndex   int       `gorm:"column:start_index" json:"start_index"`
	TotalFetched int       `gorm:"column:total_fetched" json:"total_fetched"`
	Status       string    `gorm:"column:status;type:varchar(32)" json:"status"`
}