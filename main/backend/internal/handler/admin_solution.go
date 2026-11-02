package handler

import (
	"strconv"
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
)

// AdminListSolutions 分页返回全部题解，供后台审核。
// status 可传 pending / approved / rejected，留空为全部；keyword 匹配标题。
func (h *Handler) AdminListSolutions(c *gin.Context) {
	page, pageSize, offset := httpx.Pagination(c)
	query := h.DB.Model(&model.Solution{})
	switch status := strings.TrimSpace(c.Query("status")); status {
	case model.SolutionStatusPending, model.SolutionStatusApproved, model.SolutionStatusRejected:
		query = query.Where("status = ?", status)
	}
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		query = query.Where("title LIKE ?", "%"+kw+"%")
	}
	if raw := strings.TrimSpace(c.Query("problem_id")); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			httpx.BadRequest(c, "题目参数非法")
			return
		}
		query = query.Where("problem_id = ?", id)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		httpx.ServerError(c, "查询题解失败")
		return
	}
	var rows []model.Solution
	if err := query.Order("id desc").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		httpx.ServerError(c, "查询题解失败")
		return
	}

	problemIDs := make([]uint, 0, len(rows))
	userIDs := make([]uint, 0, len(rows))
	for i := range rows {
		problemIDs = append(problemIDs, rows[i].ProblemID)
		userIDs = append(userIDs, rows[i].UserID)
	}
	problems := map[uint]model.Problem{}
	if len(problemIDs) > 0 {
		var found []model.Problem
		// 题目被删除后，题解里仍显示原来的题号与标题。
		h.DB.Unscoped().Where("id IN ?", problemIDs).Find(&found)
		for _, p := range found {
			problems[p.ID] = p
		}
	}
	users := h.communityUsers(userIDs)

	items := make([]gin.H, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		p := problems[row.ProblemID]
		u := users[row.UserID]
		items = append(items, gin.H{
			"id":                 row.ID,
			"problem_id":         row.ProblemID,
			"problem_display_id": p.DisplayID,
			"problem_title":      p.Title,
			"title":              row.Title,
			"content":            row.Content,
			"status":             row.Status,
			"review_note":        row.ReviewNote,
			"user_id":            row.UserID,
			"username":           u.Username,
			"nickname":           u.Nickname,
			"visible":            row.Visible,
			"created_at":         row.CreatedAt,
			"updated_at":         row.UpdatedAt,
			"reviewed_at":        row.ReviewedAt,
			"reviewed_by":        row.ReviewedBy,
		})
	}
	httpx.OK(c, httpx.List(total, page, pageSize, items))
}

// AdminReviewSolution 审核题解：通过或驳回，可附说明。
func (h *Handler) AdminReviewSolution(c *gin.Context) {
	u := httpx.MustUser(c)
	id, ok := httpx.PathID(c, "id")
	if u == nil || !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "参数格式错误")
		return
	}
	status := strings.TrimSpace(req.Status)
	if status != model.SolutionStatusApproved && status != model.SolutionStatusRejected {
		httpx.BadRequest(c, "审核结果只能是 approved 或 rejected")
		return
	}
	var row model.Solution
	if err := h.DB.First(&row, id).Error; err != nil {
		httpx.NotFound(c, "题解不存在")
		return
	}
	note := strings.TrimSpace(req.Note)
	if len([]rune(note)) > 255 {
		httpx.BadRequest(c, "审核说明不能超过 255 个字符")
		return
	}
	now := time.Now()
	if err := h.DB.Model(&row).Updates(map[string]any{
		"status":      status,
		"review_note": note,
		"reviewed_by": u.ID,
		"reviewed_at": now,
	}).Error; err != nil {
		httpx.ServerError(c, "保存审核结果失败")
		return
	}
	logger.Adminf("管理员 %s 审核题解 %d（题目 %d）为 %s ip=%s", u.Username, row.ID, row.ProblemID, status, c.ClientIP())
	httpx.OK(c, gin.H{"id": row.ID, "status": status})
}
