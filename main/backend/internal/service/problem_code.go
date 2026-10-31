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
func NextProblemDisplayID(db *gorm.DB, bank *model.ProblemBank) (string, error) {
	var ids []string
	if err := db.Model(&model.Problem{}).Where("bank_id = ?", bank.ID).Pluck("display_id", &ids).Error; err != nil {
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
