package handler

import (
	"akioi/internal/httpx"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
)

// AdminJudgeNodes 返回各评测节点的连接状况与数据同步情况。
func (h *Handler) AdminJudgeNodes(c *gin.Context) {
	nodes := h.Judge.CheckNodes(c.Request.Context())
	var totalWithData int64
	if err := h.DB.Model(&model.Problem{}).Where("testcase_count > 0").Count(&totalWithData).Error; err != nil {
		httpx.ServerError(c, "统计题目失败")
		return
	}
	httpx.OK(c, gin.H{
		"nodes":             nodes,
		"total_problems":    totalWithData,
		"secret_configured": h.Cfg.Judge.SyncSecret != "",
	})
}

// AdminJudgeSyncAll 把全部带测试数据的题目推送到所有节点，返回成功与失败明细。
func (h *Handler) AdminJudgeSyncAll(c *gin.Context) {
	if len(h.Settings.Current().JudgeNodeEndpoints) == 0 {
		httpx.BadRequest(c, "未配置评测节点，请先在系统设置中添加")
		return
	}

	var ids []uint
	if err := h.DB.Model(&model.Problem{}).Where("testcase_count > 0").Pluck("id", &ids).Error; err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}

	type failedItem struct {
		ProblemID uint     `json:"problem_id"`
		Nodes     []string `json:"nodes"`
	}
	success := 0
	failed := make([]failedItem, 0)
	for _, id := range ids {
		if nodes := h.Testdata.SyncToNodes(syncContext(c), id); len(nodes) > 0 {
			failed = append(failed, failedItem{ProblemID: id, Nodes: nodes})
		} else {
			success++
		}
	}
	httpx.OK(c, gin.H{"total": len(ids), "success": success, "failed": failed})
}
