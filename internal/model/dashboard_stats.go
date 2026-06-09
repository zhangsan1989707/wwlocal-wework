package model

import "time"

// DashboardDailyStat 预计算的每日看板指标
type DashboardDailyStat struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	StatDate     string    `gorm:"column:stat_date;type:date;not null;uniqueIndex:uk_date_type_key" json:"stat_date"`
	MetricType   string    `gorm:"column:metric_type;type:varchar(32);not null;uniqueIndex:uk_date_type_key" json:"metric_type"`
	DimensionKey string    `gorm:"column:dimension_key;type:varchar(128);not null;default:*;uniqueIndex:uk_date_type_key" json:"dimension_key"`
	MetricValue  int64     `gorm:"column:metric_value;not null;default:0" json:"metric_value"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (DashboardDailyStat) TableName() string {
	return "dashboard_daily_stats"
}

// DashboardDailyUserList 每日用户明细列表（用于导出和明细查询）
type DashboardDailyUserList struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	StatDate   string    `gorm:"column:stat_date;type:date;not null;uniqueIndex:uk_date_type_mobile" json:"stat_date"`
	ListType   string    `gorm:"column:list_type;type:varchar(32);not null;uniqueIndex:uk_date_type_mobile" json:"list_type"`
	Mobile     string    `gorm:"column:mobile;type:varchar(32);not null;uniqueIndex:uk_date_type_mobile" json:"mobile"`
	UserID     string    `gorm:"column:user_id;type:varchar(64)" json:"user_id"`
	Name       string    `gorm:"column:name;type:varchar(64)" json:"name"`
	Department string    `gorm:"column:department;type:varchar(255)" json:"department"`
	Extra      string    `gorm:"column:extra;type:json" json:"extra"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (DashboardDailyUserList) TableName() string {
	return "dashboard_daily_user_list"
}

// MetricType 枚举常量
const (
	MetricUserRegistered   = "user_registered"
	MetricUserActivated    = "user_activated"
	MetricUserNotActivated = "user_not_activated"
	MetricUserActive       = "user_active"
	MetricUserInactive     = "user_inactive"
	MetricRateActivation   = "rate_activation"
	MetricRateActive       = "rate_active"
	MetricMsgCount         = "msg_count"
	MetricMsgSender        = "msg_sender"
	MetricGroupCreated     = "group_created"
	MetricGroupActive      = "group_active"
	MetricRateGroupActive  = "rate_group_active"
	MetricDeviceTotal      = "device_total"
	MetricDeviceAndroid    = "device_android"
	MetricDeviceIOS        = "device_ios"
	MetricDeviceIPad       = "device_ipad"
	MetricDeviceWindows    = "device_windows"
	MetricDeviceMacOS      = "device_macos"
	MetricDeviceLinux      = "device_linux"
	MetricAppAccessUser    = "app_access_user"
	MetricAppAccessCount   = "app_access_count"
)

// ListType 枚举常量
const (
	ListTypeInactive = "inactive"
	ListTypeActive   = "active"
	ListTypeNoLogin  = "no_login"
)

// DeviceTypeMap devtype 到 metric_type 的映射
var DeviceTypeMap = map[int]string{
	131073: MetricDeviceAndroid,
	131074: MetricDeviceIOS,
	131075: MetricDeviceIPad,
	65537:  MetricDeviceWindows,
	65538:  MetricDeviceMacOS,
	65540:  MetricDeviceLinux,
}

// DeviceTypeName 设备类型中文名
var DeviceTypeName = map[string]string{
	MetricDeviceAndroid: "Android",
	MetricDeviceIOS:     "iOS",
	MetricDeviceIPad:    "iPad",
	MetricDeviceWindows: "Windows",
	MetricDeviceMacOS:   "MacOS",
	MetricDeviceLinux:   "Linux/信创",
}
