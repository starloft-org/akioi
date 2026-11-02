package handler

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

// InternalJudgement 接收评测节点回传的结果。用同步密钥鉴权，不走用户登录态。
func (h *Handler) InternalJudgement(c *gin.Context) {
	given := c.GetHeader("X-Sync-Secret")
	expected := h.Settings.Secrets().JudgeSyncSecret
	if expected == "" ||
		subtle.ConstantTimeCompare([]byte(expected), []byte(given)) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "同步密钥不正确"})
		return
	}

	var req service.JudgeResultPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "评测结果格式错误")
		return
	}
	if req.SubmissionID == 0 || req.JudgeToken == "" {
		httpx.BadRequest(c, "评测结果缺少必要字段")
		return
	}

	resultsJSON := "[]"
	if req.Results != nil {
		if raw, err := json.Marshal(req.Results); err == nil {
			resultsJSON = string(raw)
		}
	}

	res := h.DB.Model(&model.Submission{}).
		Where("id = ? AND judge_token = ?", req.SubmissionID, req.JudgeToken).
		Updates(map[string]any{
			"status":           req.Status,
			"score":            req.Score,
			"time_ms":          req.TimeMS,
			"memory_kb":        req.MemoryKB,
			"compile_error":    req.CompileError,
			"judge_message":    req.JudgeMessage,
			"testcase_results": resultsJSON,
			"judged_at":        time.Now(),
		})
	if res.Error != nil {
		logger.Errorf("保存评测结果失败 submission=%d: %v", req.SubmissionID, res.Error)
		httpx.ServerError(c, "保存评测结果失败")
		return
	}
	if res.RowsAffected == 0 {
		// 提交已被重测覆盖，丢弃这次迟到的结果。
		httpx.OK(c, gin.H{"accepted": false})
		return
	}

	if req.Status == model.StatusAccepted {
		if err := h.DB.Exec(`UPDATE problems SET accepted_count = (
			SELECT COUNT(DISTINCT s.user_id) FROM submissions s
			WHERE s.problem_id = problems.id AND s.status = ?)
			WHERE id = (SELECT problem_id FROM submissions WHERE id = ?)`,
			model.StatusAccepted, req.SubmissionID).Error; err != nil {
			logger.Errorf("重算题目通过人数失败 submission=%d: %v", req.SubmissionID, err)
		}
	}

	// 结果已经落库，进度键没有用了，顺手清掉，避免页面读到过期进度
	_ = h.RDB.Del(c.Request.Context(), service.JudgeProgressKey(req.SubmissionID)).Err()
	logger.Businessf("收到评测结果 submission=%d status=%s score=%d", req.SubmissionID, req.Status, req.Score)
	httpx.OK(c, gin.H{"accepted": true})
}

// InternalJudgeProgress 接收评测节点上报的进度：把提交标记为评测中，
// 并把「已测 x / 共 n 个测试点」写进 Redis，供页面轮询显示。
func (h *Handler) InternalJudgeProgress(c *gin.Context) {
	given := c.GetHeader("X-Sync-Secret")
	expected := h.Settings.Secrets().JudgeSyncSecret
	if expected == "" ||
		subtle.ConstantTimeCompare([]byte(expected), []byte(given)) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "同步密钥不正确"})
		return
	}

	var req service.JudgeProgressPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "评测进度格式错误")
		return
	}
	if req.SubmissionID == 0 || req.JudgeToken == "" {
		httpx.BadRequest(c, "评测进度缺少必要字段")
		return
	}

	var sub model.Submission
	if err := h.DB.Select("id", "status", "judge_token").First(&sub, req.SubmissionID).Error; err != nil {
		httpx.OK(c, gin.H{"accepted": false})
		return
	}
	// 令牌对不上说明这条提交已经被重测覆盖，这份进度来自旧任务，直接丢弃
	if sub.JudgeToken != req.JudgeToken {
		httpx.OK(c, gin.H{"accepted": false})
		return
	}
	// Pending 是排队中，节点开始评测后转成 Judging
	if sub.Status == model.StatusPending {
		res := h.DB.Model(&model.Submission{}).
			Where("id = ? AND status = ?", req.SubmissionID, model.StatusPending).
			Update("status", model.StatusJudging)
		if res.Error != nil {
			logger.Errorf("更新评测状态失败 submission=%d: %v", req.SubmissionID, res.Error)
			httpx.ServerError(c, "保存评测进度失败")
			return
		}
	}

	payload, err := json.Marshal(gin.H{
		"done":       req.Done,
		"total":      req.Total,
		"updated_at": time.Now().Unix(),
	})
	if err == nil {
		_ = h.RDB.Set(c.Request.Context(), service.JudgeProgressKey(req.SubmissionID), payload, 30*time.Minute).Err()
	}
	httpx.OK(c, gin.H{"accepted": true})
}
