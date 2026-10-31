package service

import (
	"strconv"
	"strings"

	"akioi/internal/model"
	"gorm.io/gorm"
)

// FormatProblemDisplayID 按题库前缀与序号生成题号，序号不足位宽时左补零。
func FormatProblemDisplayID(bank *model.ProblemBank, seq int) string {
	digits := strconv.Itoa(seq)
	if width := bank.CodeLength; width > len(digits) {
		digits = strings.Repeat("0", width-len(digits)) + digits
	}
	return bank.Prefix + digits
}

// ProblemDisplayIDSeq 取题号中前缀之后的序号；前缀不匹配或后缀不是纯数字时返回 false。
func ProblemDisplayIDSeq(displayID, prefix string) (int, bool) {
	if prefix == "" || !strings.HasPrefix(displayID, prefix) {
		return 0, false
	}
	digits := strings.TrimPrefix(displayID, prefix)
	if digits == "" {
		return 0, false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	seq, err := strconv.Atoi(digits)
	if err != nil || seq < 0 {
		return 0, false
	}
	return seq, true
}

// NextProblemDisplayID 返回题库内的下一个题号：取现有最大序号加一，空题库从 0 开始。
// 统计包含已删除的题目，题号删除后不会被再次分配。
func NextProblemDisplayID(db *gorm.DB, bank *model.ProblemBank) (string, error) {
	var ids []string
	if err := db.Unscoped().Model(&model.Problem{}).Where("bank_id = ?", bank.ID).Pluck("display_id", &ids).Error; err != nil {
		return "", err
	}
	next := 0
	for _, id := range ids {
		if seq, ok := ProblemDisplayIDSeq(id, bank.Prefix); ok && seq >= next {
			next = seq + 1
		}
	}
	return FormatProblemDisplayID(bank, next), nil
}

// ValidProblemDisplayID 校验题号符合题库的「前缀 + 至少 CodeLength 位数字」格式。
func ValidProblemDisplayID(bank *model.ProblemBank, displayID string) bool {
	seq, ok := ProblemDisplayIDSeq(displayID, bank.Prefix)
	if !ok || seq < 0 {
		return false
	}
	return len(displayID) >= len(bank.Prefix)+bank.CodeLength
}

// RenumberProblemDisplayIDs 按题库调整后的前缀与题号长度重排已有题号：
// 题号内的序号保持不变，只替换前缀与补零位数；序号无法解析或与其他题号重复时
// 按出现顺序顺延到下一个空闲序号。返回的题号与传入顺序一一对应。
func RenumberProblemDisplayIDs(bank *model.ProblemBank, oldPrefix string, displayIDs []string) []string {
	seqs := make([]int, len(displayIDs))
	used := make(map[int]bool, len(displayIDs))
	maxSeq := -1
	for i, displayID := range displayIDs {
		seq, ok := ProblemDisplayIDSeq(displayID, oldPrefix)
		if !ok || used[seq] {
			seqs[i] = -1
			continue
		}
		seqs[i] = seq
		used[seq] = true
		if seq > maxSeq {
			maxSeq = seq
		}
	}
	for i, seq := range seqs {
		if seq >= 0 {
			continue
		}
		next := maxSeq + 1
		for used[next] {
			next++
		}
		seqs[i] = next
		used[next] = true
		maxSeq = next
	}
	ids := make([]string, len(seqs))
	for i, seq := range seqs {
		ids[i] = FormatProblemDisplayID(bank, seq)
	}
	return ids
}
