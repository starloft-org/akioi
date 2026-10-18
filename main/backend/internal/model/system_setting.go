package model

import "time"

// SystemSetting 保存可在管理后台修改的运行时设置。
// Value 使用 JSON 是为了让一次保存具备原子性，也便于后续增加设置项。
type SystemSetting struct {
	Name      string    `gorm:"column:setting_key;size:64;primaryKey" json:"name"`
	Value     string    `gorm:"type:longtext;not null" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemSetting) TableName() string { return "system_settings" }
