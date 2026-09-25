package handler

import (
	"akioi/internal/httpx"

	"github.com/gin-gonic/gin"
)

// AdminListSubmissions 返回全站提交列表。
func (h *Handler) AdminListSubmissions(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	items, total, page, pageSize, err := h.querySubmissions(c, u, 0)
	if err != nil {
		httpx.ServerError(c, "查询提交失败")
		return
	}
	httpx.OK(c, httpx.List(total, page, pageSize, items))
}

// AdminRejudgeSubmission 重测单条提交。
func (h *Handler) AdminRejudgeSubmission(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	count, err := h.Judge.Rejudge(c.Request.Context(), []uint{id})
	if err != nil {
		httpx.ServerError(c, "重测失败")
		return
	}
	httpx.OK(c, gin.H{"count": count})
}

type adminRejudgeRequest struct {
	IDs       []uint `json:"ids"`
	ProblemID uint   `json:"problem_id"`
	ContestID uint   `json:"contest_id"`
}

// AdminRejudge 按提交 ID 列表、题目或比赛批量重测。
func (h *Handler) AdminRejudge(c *gin.Context) {
	var req adminRejudgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}

	ctx := c.Request.Context()
	var count int
	var err error
	switch {
	case len(req.IDs) > 0:
		count, err = h.Judge.Rejudge(ctx, req.IDs)
	case req.ProblemID > 0:
		count, err = h.Judge.RejudgeProblem(ctx, req.ProblemID)
	case req.ContestID > 0:
		count, err = h.Judge.RejudgeContest(ctx, req.ContestID)
	default:
		httpx.BadRequest(c, "请指定要重测的提交、题目或比赛")
		return
	}
	if err != nil {
		httpx.ServerError(c, "重测失败")
		return
	}
	httpx.OK(c, gin.H{"count": count})
}
