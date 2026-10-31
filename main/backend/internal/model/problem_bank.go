package model

import "time"

// ProblemBank 是题号的分配容器：题号 = 前缀 + 定长序号，序号在题库内自动递增。
type ProblemBank struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	Name       string `gorm:"size:64;not null" json:"name"`
	Prefix     string `gorm:"size:8;uniqueIndex;not null" json:"prefix"`
	CodeLength int    `gorm:"not null;default:4" json:"code_length"`
	// Role 标记预置题库的角色，后台自建题库为空值。
	Role      string    `gorm:"column:preset_role;size:16;not null;default:''" json:"role"`
	Remark    string    `gorm:"size:255" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ProblemBank) TableName() string { return "problem_banks" }

// IsPreset 判断是否为迁移预置的主库、团队库或个人库。
func (b ProblemBank) IsPreset() bool { return b.Role != "" }
