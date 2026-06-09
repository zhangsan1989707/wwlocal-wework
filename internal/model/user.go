package model

import "time"

const (
	RoleSuperAdmin = "super_admin"
	RoleDeptAdmin  = "dept_admin"
)

type User struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"column:username;type:varchar(64);not null;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"column:password_hash;type:varchar(255);not null" json:"-"`
	Role         string    `gorm:"column:role;type:varchar(32);not null;default:dept_admin" json:"role"`
	Enabled      bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

type UserDeptScope struct {
	UserID    int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
	DeptID    int       `gorm:"primaryKey;column:dept_id" json:"dept_id"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (UserDeptScope) TableName() string {
	return "user_dept_scopes"
}
