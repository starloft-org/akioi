package handler

import (
	"akioi/internal/model"

	"gorm.io/gorm"
)

// visibleProblemIDs 返回「该用户有权查看提交」的题目子查询：
// 公共题、自己上传的个人题、所在团队里的团队题。隐藏题一律不包含，
// 避免借提交列表或他人主页读到私有题内容与评测细节。
func (h *Handler) visibleProblemIDs(u *model.User) *gorm.DB {
	sub := h.DB.Model(&model.Problem{}).Select("id").
		Where("visibility = ?", model.ProblemVisibilityPublic).
		Where("team_id = 0 AND owner_id = 0 AND access IN ?",
			[]int{model.ProblemAccessPublic, model.ProblemAccessMembers})
	if u != nil {
		teamIDs := h.DB.Model(&model.TeamMember{}).Select("team_id").Where("user_id = ?", u.ID)
		sub = sub.Or("owner_id = ?", u.ID).Or("team_id IN (?)", teamIDs)
	}
	return sub
}

// submissionVisibleTo 判断请求者能否看到这条提交（含别人的提交）。
func (h *Handler) submissionVisibleTo(u *model.User, sub *model.Submission) bool {
	if u == nil {
		return false
	}
	if u.IsAdmin() || sub.UserID == u.ID {
		return true
	}
	if sub.ContestID > 0 && !h.contestRevealed(sub.ContestID) {
		return false
	}
	var count int64
	if err := h.DB.Model(&model.Problem{}).
		Where("id = ?", sub.ProblemID).
		Where("id IN (?)", h.visibleProblemIDs(u)).
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}
