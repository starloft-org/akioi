package model

import "time"

// JudgeJob 是评测节点本地保存的任务记录。节点使用独立数据库，
// 不再访问主站的 submissions / problems 表。
type JudgeJob struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SubmissionID uint      `gorm:"uniqueIndex;not null" json:"submission_id"`
	JudgeToken   string    `gorm:"size:64;not null" json:"judge_token"`
	Payload      string    `gorm:"type:mediumtext;not null" json:"-"`
	Status       string    `gorm:"size:16;not null;default:pending" json:"status"`
	Result       string    `gorm:"type:mediumtext" json:"-"`
	Synced       bool      `gorm:"not null;default:false" json:"synced"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (JudgeJob) TableName() string { return "judge_jobs" }

// JudgeTask 是主站下发的完整评测任务，评测所需数据全部自带。
type JudgeTask struct {
	SubmissionID  uint           `json:"submission_id"`
	JudgeToken    string         `json:"judge_token"`
	ProblemID     uint           `json:"problem_id"`
	Language      string         `json:"language"`
	Code          string         `json:"code"`
	TimeLimitMS   int            `json:"time_limit_ms"`
	MemoryLimitMB int            `json:"memory_limit_mb"`
	CompareMode   string         `json:"compare_mode"`
	FloatEps      float64        `json:"float_eps"`
	Testcases     []TaskTestcase `json:"testcases"`
}

// TaskTestcase 描述一个测试点在本地数据目录中的序号与分值。
type TaskTestcase struct {
	Index int `json:"index"`
	Score int `json:"score"`
}

// JudgeResult 是评测节点回传给主站的结果。
type JudgeResult struct {
	SubmissionID uint             `json:"submission_id"`
	JudgeToken   string           `json:"judge_token"`
	Status       string           `json:"status"`
	Score        int              `json:"score"`
	TimeMS       int              `json:"time_ms"`
	MemoryKB     int              `json:"memory_kb"`
	CompileError string           `json:"compile_error"`
	JudgeMessage string           `json:"judge_message"`
	Results      []TestcaseResult `json:"testcase_results"`
}
