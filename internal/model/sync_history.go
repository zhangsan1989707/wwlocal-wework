package model

import "time"

type SyncHistory struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SyncType    string    `gorm:"type:varchar(32);index:idx_type" json:"sync_type"` // log / contact
	Trigger     string    `gorm:"type:varchar(16)" json:"trigger"`                  // manual / scheduler
	StartTime   time.Time `gorm:"index:idx_start" json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	DurationMs  int64     `json:"duration_ms"`
	Total       int       `json:"total"`       // feature 总数（日志同步）或用户总数（通讯录同步）
	Succeeded   int       `json:"succeeded"`   // 成功数量
	Failed      int       `json:"failed"`      // 失败数量
	Details     string    `gorm:"type:TEXT" json:"details"` // JSON: 每个 feature 的同步结果
	ErrorMsg     string    `gorm:"type:TEXT" json:"error_msg"`
}

func (SyncHistory) TableName() string {
	return "sync_history"
}
