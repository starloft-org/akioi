package model

import "time"

// TestcaseResult 是单个测试点的判题结果，整体以 JSON 存入 submissions.testcase_results。
type TestcaseResult struct {
	Index    int    `json:"index"`
	Status   string `json:"status"`
	TimeMS   int    `json:"time_ms"`
	MemoryKB int    `json:"memory_kb"`
	Score    int    `json:"score"`
	// Group 是所属子任务编号，0 表示独立测试点。
	Group   int    `json:"group,omitempty"`
	Message string `json:"message"`
	// Stderr 保留该测试点的错误输出，便于在评测详情里查看完整原因。
	Stderr string `json:"stderr,omitempty"`
	// ExitCode 为选手程序的退出码，0 表示未记录或正常结束。
	ExitCode int `json:"exit_code,omitempty"`
}

type Submission struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"index;not null" json:"user_id"`
	ProblemID    uint       `gorm:"index;not null" json:"problem_id"`
	ContestID    uint       `gorm:"index;not null;default:0" json:"contest_id"`
	Language     string     `gorm:"size:16;not null" json:"language"`
	Code         string     `gorm:"type:mediumtext" json:"-"`
	CodeLength   int        `gorm:"not null;default:0" json:"code_length"`
	Status       string     `gorm:"size:32;index;not null;default:Pending" json:"status"`
	Score        int        `gorm:"not null;default:0" json:"score"`
	TimeMS       int        `gorm:"not null;default:0" json:"time_ms"`
	MemoryKB     int        `gorm:"not null;default:0" json:"memory_kb"`
	CompileError string     `gorm:"type:text" json:"compile_error"`
	ResultsJSON  string     `gorm:"column:testcase_results;type:mediumtext" json:"-"`
	JudgeToken   string     `gorm:"size:64" json:"-"`
	JudgeMessage string     `gorm:"size:255" json:"judge_message"`
	JudgedAt     *time.Time `json:"judged_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (Submission) TableName() string { return "submissions" }

// Results 返回逐测试点结果。
func (s *Submission) Results() []TestcaseResult {
	var out []TestcaseResult
	unmarshalJSON(s.ResultsJSON, &out)
	if out == nil {
		out = []TestcaseResult{}
	}
	return out
}

func (s *Submission) SetResults(list []TestcaseResult) {
	if list == nil {
		list = []TestcaseResult{}
	}
	s.ResultsJSON = marshalJSON(list)
}
