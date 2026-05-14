package model

type UserDailyStats struct {
	Mobile    string `gorm:"column:mobile;type:varchar(32);primaryKey" json:"mobile"`
	FeatureID int    `gorm:"column:feature_id;primaryKey" json:"feature_id"`
	StatDate  string `gorm:"column:stat_date;type:date;primaryKey" json:"stat_date"`
}
