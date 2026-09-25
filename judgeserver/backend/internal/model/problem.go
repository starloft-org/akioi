package model

import "time"

// Sample 是题面样例的一组输入输出。
type Sample struct {
	Input       string `json:"input"`
	Output      string `json:"output"`
	Explanation string `json:"explanation"`
}

type Problem struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	DisplayID     string    `gorm:"size:16;uniqueIndex;not null" json:"display_id"`
	Title         string    `gorm:"size:128;not null" json:"title"`
	Description   string    `gorm:"type:mediumtext" json:"description"`
	InputFormat   string    `gorm:"type:mediumtext" json:"input_format"`
	OutputFormat  string    `gorm:"type:mediumtext" json:"output_format"`
	Hint          string    `gorm:"type:mediumtext" json:"hint"`
	SamplesJSON   string    `gorm:"column:samples;type:text" json:"-"`
	TimeLimitMS   int       `gorm:"not null;default:1000" json:"time_limit_ms"`
	MemoryLimitMB int       `gorm:"not null;default:256" json:"memory_limit_mb"`
	Difficulty    string    `gorm:"size:16;index" json:"difficulty"`
	TagsJSON      string    `gorm:"column:tags;size:255" json:"-"`
	Source        string    `gorm:"size:128" json:"source"`
	Visibility    int       `gorm:"not null;default:1" json:"visibility"`
	ContestOnly   bool      `gorm:"not null;default:false" json:"contest_only"`
	CompareMode   string    `gorm:"size:16;not null;default:default" json:"compare_mode"`
	FloatEps      float64   `gorm:"not null;default:0.000001" json:"float_eps"`
	TestcaseCount int       `gorm:"not null;default:0" json:"testcase_count"`
	ScoresJSON    string    `gorm:"column:testcase_scores;type:text" json:"-"`
	DataHash      string    `gorm:"size:64" json:"data_hash"`
	SubmitCount   int       `gorm:"not null;default:0" json:"submit_count"`
	AcceptedCount int       `gorm:"not null;default:0" json:"accepted_count"`
	CreatedBy     uint      `gorm:"index" json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Problem) TableName() string { return "problems" }

// Samples 返回题面样例列表。
func (p *Problem) Samples() []Sample {
	var out []Sample
	unmarshalJSON(p.SamplesJSON, &out)
	if out == nil {
		out = []Sample{}
	}
	return out
}

func (p *Problem) SetSamples(list []Sample) {
	if list == nil {
		list = []Sample{}
	}
	p.SamplesJSON = marshalJSON(list)
}

// Tags 返回题目标签列表。
func (p *Problem) Tags() []string {
	var out []string
	unmarshalJSON(p.TagsJSON, &out)
	if out == nil {
		out = []string{}
	}
	return out
}

func (p *Problem) SetTags(list []string) {
	if list == nil {
		list = []string{}
	}
	p.TagsJSON = marshalJSON(list)
}

// Scores 返回各测试点分值，下标从 0 对应测试点 1。
func (p *Problem) Scores() []int {
	var out []int
	unmarshalJSON(p.ScoresJSON, &out)
	return out
}

func (p *Problem) SetScores(list []int) {
	if list == nil {
		list = []int{}
	}
	p.ScoresJSON = marshalJSON(list)
}

// Testcase 是测试点的元数据，输入输出文件落文件系统。
// 数据库列名用 case_index，避免与 MySQL 保留字 index 冲突。
type Testcase struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ProblemID  uint      `gorm:"uniqueIndex:uk_problem_index;not null" json:"problem_id"`
	Index      int       `gorm:"column:case_index;uniqueIndex:uk_problem_index;not null" json:"index"`
	Score      int       `gorm:"not null;default:0" json:"score"`
	InputSize  int64     `gorm:"not null;default:0" json:"input_size"`
	OutputSize int64     `gorm:"not null;default:0" json:"output_size"`
	InputHash  string    `gorm:"size:64" json:"input_hash"`
	OutputHash string    `gorm:"size:64" json:"output_hash"`
	CreatedAt  time.Time `json:"created_at"`
}

func (Testcase) TableName() string { return "testcases" }
