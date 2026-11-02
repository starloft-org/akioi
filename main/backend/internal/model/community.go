package model

import (
	"time"

	"gorm.io/gorm"
)

// Solution 是用户针对题目发布的题解。
type Solution struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	ProblemID  uint           `gorm:"index;not null" json:"problem_id"`
	UserID     uint           `gorm:"index;not null" json:"user_id"`
	Title      string         `gorm:"size:128;not null" json:"title"`
	Content    string         `gorm:"type:mediumtext;not null" json:"content"`
	Visible    bool           `gorm:"index;not null;default:true" json:"visible"`
	Status     string         `gorm:"size:16;index;not null;default:approved" json:"status"`
	ReviewNote string         `gorm:"size:255;not null;default:''" json:"review_note"`
	ReviewedBy uint           `gorm:"not null;default:0" json:"reviewed_by"`
	ReviewedAt *time.Time     `json:"reviewed_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// 题解的审核状态。新发布的题解默认待审核，管理员通过后才会公开展示。
const (
	SolutionStatusPending  = "pending"
	SolutionStatusApproved = "approved"
	SolutionStatusRejected = "rejected"
)

func (Solution) TableName() string { return "solutions" }

// Discussion 是全站讨论主题，可选关联一道题目。
type Discussion struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ProblemID    uint           `gorm:"index;not null;default:0" json:"problem_id"`
	UserID       uint           `gorm:"index;not null" json:"user_id"`
	Title        string         `gorm:"size:128;not null" json:"title"`
	Content      string         `gorm:"type:mediumtext;not null" json:"content"`
	Visible      bool           `gorm:"index;not null;default:true" json:"visible"`
	CommentCount int            `gorm:"not null;default:0" json:"comment_count"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

func (Discussion) TableName() string { return "discussions" }

// DiscussionComment 是讨论主题下的评论。
type DiscussionComment struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	DiscussionID uint           `gorm:"index;not null" json:"discussion_id"`
	UserID       uint           `gorm:"index;not null" json:"user_id"`
	Content      string         `gorm:"type:text;not null" json:"content"`
	Visible      bool           `gorm:"index;not null;default:true" json:"visible"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

func (DiscussionComment) TableName() string { return "discussion_comments" }
