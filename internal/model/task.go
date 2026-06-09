package model

import "time"

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

type TaskType string

const (
	TaskTypeLogSync     TaskType = "log_sync"
	TaskTypeContactSync TaskType = "contact_sync"
	TaskTypeAdminLogSync TaskType = "admin_log_sync"
)

type SyncTask struct {
	ID         string                 `json:"id"`
	Type       TaskType               `json:"type"`
	FeatureIDs []int                  `json:"feature_ids,omitempty"`
	StartTime  int64                  `json:"start_time"`
	EndTime    int64                  `json:"end_time"`
	Status     TaskStatus             `json:"status"`
	Progress   int                    `json:"progress"`
	Total      int                    `json:"total"`
	Error      string                 `json:"error,omitempty"`
	Result     map[string]interface{} `json:"result,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}
