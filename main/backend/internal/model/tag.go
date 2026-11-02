package model

import (
	"strings"
	"time"
)

// MaxProblemTags 是一道题目最多可以带多少个标签。
const MaxProblemTags = 20

// MaxProblemTagRunes 是单个标签的最大长度（按字符计）。
const MaxProblemTagRunes = 24

// Tag 是题目标签目录，按名称唯一；题目与标签之间通过 ProblemTag 关联。
type Tag struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Name      string    `gorm:"size:64;not null;uniqueIndex:uk_tag_name" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// ProblemTag 记录题目与标签的关联，同一题目同一标签只有一条。
type ProblemTag struct {
	ProblemID uint `gorm:"primaryKey;index" json:"problem_id"`
	TagID     uint `gorm:"primaryKey;index" json:"tag_id"`
}

// NormalizeTags 清洗标签列表：去首尾空白、合并连续空白、去空、去重、限制长度与数量。
func NormalizeTags(list []string) []string {
	seen := make(map[string]bool, len(list))
	out := make([]string, 0, len(list))
	for _, raw := range list {
		name := strings.Join(strings.Fields(raw), " ")
		if name == "" {
			continue
		}
		if runes := []rune(name); len(runes) > MaxProblemTagRunes {
			name = strings.TrimSpace(string(runes[:MaxProblemTagRunes]))
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
		if len(out) >= MaxProblemTags {
			break
		}
	}
	return out
}
