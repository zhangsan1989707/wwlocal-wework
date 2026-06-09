package model

import "time"

type AdminOperLog struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OperTime   int64     `gorm:"column:oper_time;not null;index:idx_oper_time" json:"time"`
	OperTypeID int       `gorm:"column:oper_type_id" json:"oper_type_id"`
	OperType   string    `gorm:"column:oper_type;type:varchar(128)" json:"oper_type"`
	OperUserID string    `gorm:"column:oper_userid;type:varchar(64);index:idx_oper_userid" json:"oper_userid"`
	OperName   string    `gorm:"column:oper_name;type:varchar(128)" json:"oper_name"`
	OperData   string    `gorm:"column:oper_data;type:text" json:"oper_data"`
	OperDesc   string    `gorm:"column:oper_desc;type:varchar(512)" json:"oper_desc"`
	AppID      string    `gorm:"column:app_id;type:varchar(64)" json:"app_id"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (AdminOperLog) TableName() string {
	return "admin_oper_logs"
}

type AdminOperLogResponse struct {
	Errcode   int                `json:"errcode"`
	Errmsg    string             `json:"errmsg"`
	OperList  []AdminOperLogAPI `json:"oper_list"`
	NextStart int                `json:"next_start,omitempty"`
}

type AdminOperLogAPI struct {
	OperTime   int64  `json:"oper_time"`
	OperTypeID int    `json:"oper_type_id"`
	OperType   string `json:"oper_type"`
	OperUserID string `json:"oper_userid"`
	OperName   string `json:"oper_name"`
	OperData   string `json:"oper_data"`
}

type OperData struct {
	AppID   string `json:"appid"`
	Content string `json:"content"`
}
