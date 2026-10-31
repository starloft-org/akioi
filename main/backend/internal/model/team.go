package model

import (
	"encoding/json"
	"gorm.io/gorm"
	"strings"
	"time"
)

const (
	// TeamRoleOwner 队长，可审批申请、管理成员与题单。
	TeamRoleOwner = "owner"
	// TeamRoleMember 普通成员。
	TeamRoleMember = "member"

	// TeamApplicationPending 待审批。
	TeamApplicationPending = "pending"
	// TeamApplicationApproved 已通过。
	TeamApplicationApproved = "approved"
	// TeamApplicationRejected 已拒绝。
	TeamApplicationRejected = "rejected"
)

// Team 是团队，持有团队题目、题单与团队比赛。
type Team struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:32;uniqueIndex;not null" json:"name"`
	Description string         `gorm:"size:255" json:"description"`
	OwnerID     uint           `gorm:"index;not null" json:"owner_id"`
	MemberCount int            `gorm:"not null;default:1" json:"member_count"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func (Team) TableName() string { return "teams" }

// TeamMember 是团队成员关系，同一用户在同一团队仅一条记录。
type TeamMember struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	TeamID   uint      `gorm:"uniqueIndex:idx_team_user;not null" json:"team_id"`
	UserID   uint      `gorm:"uniqueIndex:idx_team_user;not null" json:"user_id"`
	Role     string    `gorm:"size:16;not null;default:member" json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

func (TeamMember) TableName() string { return "team_members" }

// TeamApplication 是加入团队的申请；同一用户可对同一团队多次申请，仅 latest 有效。
type TeamApplication struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	TeamID     uint       `gorm:"index;not null" json:"team_id"`
	UserID     uint       `gorm:"index;not null" json:"user_id"`
	Message    string     `gorm:"size:255" json:"message"`
	Status     string     `gorm:"size:16;not null;default:pending" json:"status"`
	ReviewedBy uint       `json:"reviewed_by"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (TeamApplication) TableName() string { return "team_applications" }

// TeamProblemSet 是团队内部的训练题单，仅成员可见。
type TeamProblemSet struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	TeamID         uint           `gorm:"index;not null" json:"team_id"`
	Title          string         `gorm:"size:128;not null" json:"title"`
	Description    string         `gorm:"size:255" json:"description"`
	ProblemIDsJSON string         `gorm:"column:problem_ids;type:text" json:"-"`
	CreatedBy      uint           `gorm:"index" json:"created_by"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (TeamProblemSet) TableName() string { return "team_problem_sets" }

// Problems 解析题单中的题目 ID 列表。
func (s *TeamProblemSet) Problems() []uint {
	if strings.TrimSpace(s.ProblemIDsJSON) == "" {
		return nil
	}
	var ids []uint
	_ = json.Unmarshal([]byte(s.ProblemIDsJSON), &ids)
	return ids
}

// SetProblems 写入题目 ID 列表。
func (s *TeamProblemSet) SetProblems(ids []uint) {
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
