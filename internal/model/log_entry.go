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
