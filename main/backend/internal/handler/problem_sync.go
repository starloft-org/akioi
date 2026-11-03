package handler

import (
	"context"
	"time"

	"akioi/internal/logger"
	"akioi/internal/model"
)

// problemSyncTimeout 是后台推送测试数据的兜底超时，避免节点无响应时协程一直挂着。
const problemSyncTimeout = 10 * time.Minute

// scheduleProblemSync 在后台把题目的测试数据推送到评测节点。
// 创建/编辑题目、上传测试数据之后都会自动触发一次；推送过程不与前端交互，
// 失败只写日志，由 2 分钟一次的 SyncPending 定时任务兜底重试。
func (h *Handler) scheduleProblemSync(problemID uint) {
	if problemID == 0 {
		return
	}
	if len(h.Settings.Current().JudgeNodeEndpoints) == 0 {
		return
	}
	var cases int64
	if err := h.DB.Model(&model.Testcase{}).Where("problem_id = ?", problemID).Count(&cases).Error; err != nil || cases == 0 {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("题目 %d 测试数据后台推送异常: %v", problemID, r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), problemSyncTimeout)
		defer cancel()
		if failed := h.Testdata.SyncToNodes(ctx, problemID); len(failed) > 0 {
			logger.Errorf("题目 %d 测试数据自动推送失败，节点：%v", problemID, failed)
		}
	}()
}
