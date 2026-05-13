package model

import "time"

type OperationLog struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username   string    `gorm:"type:varchar(64);index:idx_username" json:"username"`
	Action     string    `gorm:"type:varchar(32);index:idx_action" json:"action"`
	Method     string    `gorm:"type:varchar(8)" json:"method"`
	Path       string    `gorm:"type:varchar(255)" json:"path"`
	StatusCode int       `gorm:"index:idx_status" json:"status_code"`
	ErrorMsg   string    `gorm:"type:text" json:"error_msg"`
	DurationMs int64     `json:"duration_ms"`
	IP         string    `gorm:"type:varchar(46)" json:"ip"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index:idx_created_at" json:"created_at"`
}

func (OperationLog) TableName() string {
	return "operation_logs"
}
