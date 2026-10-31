package model

import (
	"encoding/json"
	"gorm.io/gorm"
	"strings"
	"time"
)

// UserProblemSet 是用户自建的训练题单，仅创建者可见。
type UserProblemSet struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	OwnerID        uint           `gorm:"index;not null" json:"owner_id"`
	Title          string         `gorm:"size:128;not null" json:"title"`
	Description    string         `gorm:"size:255" json:"description"`
	ProblemIDsJSON string         `gorm:"column:problem_ids;type:text" json:"-"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (UserProblemSet) TableName() string { return "user_problem_sets" }

// Problems 解析题单中的题目 ID 列表。
func (s *UserProblemSet) Problems() []uint {
	if strings.TrimSpace(s.ProblemIDsJSON) == "" {
		return nil
	}
	var ids []uint
	_ = json.Unmarshal([]byte(s.ProblemIDsJSON), &ids)
	return ids
}

// SetProblems 写入题目 ID 列表。
func (s *UserProblemSet) SetProblems(ids []uint) {
	if ids == nil {
		ids = []uint{}
	}
	b, err := json.Marshal(ids)
	if err != nil {
		s.ProblemIDsJSON = "[]"
		return
	}
	s.ProblemIDsJSON = string(b)
}
