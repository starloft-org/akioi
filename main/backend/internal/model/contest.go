package model

import "time"

type Contest struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Title            string    `gorm:"size:128;not null" json:"title"`
	Description      string    `gorm:"type:mediumtext" json:"description"`
	StartAt          time.Time `gorm:"index;not null" json:"start_at"`
	EndAt            time.Time `gorm:"not null" json:"end_at"`
	FreezeMinutes    int       `gorm:"not null;default:0" json:"freeze_minutes"`
	Password         string    `gorm:"size:64" json:"-"`
	RegisterRequired bool      `gorm:"not null;default:false" json:"register_required"`
	CreatedBy        uint      `gorm:"index" json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (Contest) TableName() string { return "contests" }

// Status 依据当前时间计算比赛状态。
func (c *Contest) Status(now time.Time) string {
	switch {
	case now.Before(c.StartAt):
		return ContestStatusPending
	case now.After(c.EndAt):
		return ContestStatusEnded
	default:
		return ContestStatusRunning
	}
}

// FreezeAt 返回封榜时刻；未配置封榜时返回 EndAt。
func (c *Contest) FreezeAt() time.Time {
	if c.FreezeMinutes <= 0 {
		return c.EndAt
	}
	return c.EndAt.Add(-time.Duration(c.FreezeMinutes) * time.Minute)
}

// IsFrozen 表示当前时刻处于封榜期。
func (c *Contest) IsFrozen(now time.Time) bool {
	return c.FreezeMinutes > 0 && now.After(c.FreezeAt()) && now.Before(c.EndAt)
}

type ContestProblem struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	ContestID  uint   `gorm:"uniqueIndex:uk_contest_problem;not null" json:"contest_id"`
	ProblemID  uint   `gorm:"uniqueIndex:uk_contest_problem;not null" json:"problem_id"`
	Label      string `gorm:"size:8" json:"label"`
	OrderIndex int    `gorm:"not null;default:0" json:"order_index"`
}

func (ContestProblem) TableName() string { return "contest_problems" }

type ContestParticipant struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ContestID    uint      `gorm:"uniqueIndex:uk_contest_user;not null" json:"contest_id"`
	UserID       uint      `gorm:"uniqueIndex:uk_contest_user;not null" json:"user_id"`
	RegisteredAt time.Time `json:"registered_at"`
}

func (ContestParticipant) TableName() string { return "contest_participants" }

// RankProblemCell 是榜单中某位选手在某道题上的成绩。
type RankProblemCell struct {
	ProblemID  uint   `json:"problem_id"`
	Label      string `json:"label"`
	Score      int    `json:"score"`
	Attempts   int    `json:"attempts"`
	Accepted   bool   `json:"accepted"`
	AcceptedAt int64  `json:"accepted_at"`
}

// RankItem 是榜单中的一行。
type RankItem struct {
	Rank        int               `json:"rank"`
	UserID      uint              `json:"user_id"`
	Username    string            `json:"username"`
	Nickname    string            `json:"nickname"`
	TotalScore  int               `json:"total_score"`
	AcceptedNum int               `json:"accepted_num"`
	LastSubmit  int64             `json:"last_submit"`
	Problems    []RankProblemCell `json:"problems"`
}

type Announcement struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Title     string    `gorm:"size:128;not null" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	Visible   bool      `gorm:"not null;default:true" json:"visible"`
	CreatedBy uint      `gorm:"index" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Announcement) TableName() string { return "announcements" }
