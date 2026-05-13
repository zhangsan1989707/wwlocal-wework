package model

import "time"

type Contact struct {
	UserID     string    `gorm:"primaryKey;column:user_id;type:varchar(64)" json:"userid"`
	Name       string    `gorm:"column:name;type:varchar(128)" json:"name"`
	Mobile     string    `gorm:"column:mobile;type:varchar(32);index:idx_mobile" json:"mobile"`
	Gender     int       `gorm:"column:gender;default:0" json:"gender"`
	Email      string    `gorm:"column:email;type:varchar(128)" json:"email"`
	Position   string    `gorm:"column:position;type:varchar(128)" json:"position"`
	Department string    `gorm:"column:department;type:varchar(256)" json:"department"`
	Positions  string    `gorm:"column:positions;type:text" json:"positions"`
	Avatar     string    `gorm:"column:avatar;type:varchar(512)" json:"avatar"`
	Status     int       `gorm:"column:status;default:1" json:"status"`
	RawJSON    string    `gorm:"column:raw_json;type:text" json:"raw_json"`
	SyncedAt   time.Time `gorm:"column:synced_at" json:"synced_at"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (Contact) TableName() string {
	return "contacts"
}

type Department struct {
	ID       int       `gorm:"primaryKey;column:id" json:"id"`
	Name     string    `gorm:"column:name;type:varchar(128)" json:"name"`
	ParentID int       `gorm:"column:parent_id;default:0;index:idx_parent" json:"parentid"`
	Order    int       `gorm:"column:order_num;default:0" json:"order"`
	Type     int       `gorm:"column:type;default:0" json:"type"`
	SyncedAt time.Time `gorm:"column:synced_at" json:"synced_at"`
}

func (Department) TableName() string {
	return "departments"
}

type DeptTreeNode struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	ParentID    int            `json:"parentid"`
	Order       int            `json:"order"`
	Type        int            `json:"type"`
	MemberCount int            `json:"member_count"`
	Children    []DeptTreeNode `json:"children"`
}

// API 响应 DTO

type DepartmentItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ParentID int    `json:"parentid"`
	Order    int    `json:"order"`
	Type     int    `json:"type"`
}

type SimpleUser struct {
	UserID         string   `json:"userid"`
	Name           string   `json:"name"`
	Department     []int    `json:"department"`
	IsLeaderInDept []int    `json:"is_leader_in_dept"`
	Positions      []string `json:"positions"`
}

type ContactDetail struct {
	UserID         string        `json:"userid"`
	Name           string        `json:"name"`
	Department     []int         `json:"department"`
	Order          []interface{} `json:"order,omitempty"`
	Position       string        `json:"position"`
	Positions      []string      `json:"positions"`
	Mobile         string        `json:"mobile"`
	Gender         string        `json:"gender"`
	Email          string        `json:"email"`
	IsLeaderInDept []int         `json:"is_leader_in_dept"`
	Avatar         string        `json:"avatar"`
	Telephone      string   `json:"telephone"`
	EnglishName    string   `json:"english_name"`
	Status         int      `json:"status"`
	Enable         int      `json:"enable"`
}
