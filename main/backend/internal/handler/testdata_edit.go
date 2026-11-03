package handler

import (
	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
)

// testcaseScoreInput 是修改单个测试点分值的请求项。
type testcaseScoreInput struct {
	Index int `json:"index"`
	Score int `json:"score"`
}

// AdminUpdateTestdataScores 修改后台题目各测试点的分值。
func (h *Handler) AdminUpdateTestdataScores(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return
	}
	h.updateProblemTestdataScores(c, &p)
}

// AdminReorderTestdata 调整后台题目测试点的顺序。
func (h *Handler) AdminReorderTestdata(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return
	}
	h.reorderProblemTestdata(c, &p)
}

// MyUpdateTestdataScores 修改自己题目各测试点的分值。
func (h *Handler) MyUpdateTestdataScores(c *gin.Context) {
	p, ok := h.loadManageableProblem(c)
	if !ok {
		return
	}
	h.updateProblemTestdataScores(c, p)
}

// MyReorderTestdata 调整自己题目测试点的顺序。
func (h *Handler) MyReorderTestdata(c *gin.Context) {
	p, ok := h.loadManageableProblem(c)
	if !ok {
		return
	}
	h.reorderProblemTestdata(c, p)
}

func (h *Handler) updateProblemTestdataScores(c *gin.Context, p *model.Problem) {
	var req struct {
		Scores []testcaseScoreInput `json:"scores"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "分值参数格式错误")
		return
	}
	scores := make(map[int]int, len(req.Scores))
	for _, item := range req.Scores {
		if item.Index <= 0 {
			httpx.BadRequest(c, "测试点序号非法")
			return
		}
		if item.Score < 0 {
			httpx.BadRequest(c, "测试点分值不能为负数")
			return
		}
		scores[item.Index] = item.Score
	}
	if err := h.Testdata.UpdateScores(p.ID, scores); err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	logger.Businessf("题目 %s 的测试点分值已更新", p.DisplayID)
	fresh := *p
	if err := h.DB.First(&fresh, p.ID).Error; err != nil {
		httpx.ServerError(c, "题目不存在")
		return
	}
	h.respondProblemTestdataWithSync(c, &fresh, nil)
}

func (h *Handler) reorderProblemTestdata(c *gin.Context, p *model.Problem) {
	var req struct {
		Order []int `json:"order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "排序参数格式错误")
		return
	}
	if err := h.Testdata.Reorder(p.ID, req.Order); err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	logger.Businessf("题目 %s 的测试点顺序已调整", p.DisplayID)
	failed := h.Testdata.SyncToNodes(syncContext(c), p.ID)
	fresh := *p
	if err := h.DB.First(&fresh, p.ID).Error; err != nil {
		httpx.ServerError(c, "题目不存在")
		return
	}
	h.respondProblemTestdataWithSync(c, &fresh, failed)
}

// respondProblemTestdataWithSync 返回测试点清单并附带测试数据同步失败的节点。
func (h *Handler) respondProblemTestdataWithSync(c *gin.Context, p *model.Problem, syncFailed []string) {
	items, err := h.testcaseItems(p.ID)
	if err != nil {
		httpx.ServerError(c, "查询测试数据失败")
		return
	}
	total := 0
	for _, item := range items {
		total += item.Score
	}
	httpx.OK(c, gin.H{
		"items":       items,
		"total_score": total,
		"subtasks":    p.Subtasks(),
		"sync_failed": syncFailed,
	})
}
