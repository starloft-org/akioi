package handler

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/model"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

// SubmissionItem 是提交列表与详情共用的视图，不含源码。
type SubmissionItem struct {
	ID               uint       `json:"id"`
	UserID           uint       `json:"user_id"`
	Username         string     `json:"username"`
	Nickname         string     `json:"nickname"`
	ProblemID        uint       `json:"problem_id"`
	ProblemDisplayID string     `json:"problem_display_id"`
	ProblemTitle     string     `json:"problem_title"`
	ContestID        uint       `json:"contest_id"`
	Language         string     `json:"language"`
	Status           string     `json:"status"`
	Score            int        `json:"score"`
	TimeMS           int        `json:"time_ms"`
	MemoryKB         int        `json:"memory_kb"`
	CodeLength       int        `json:"code_length"`
	CreatedAt        time.Time  `json:"created_at"`
	JudgedAt         *time.Time `json:"judged_at"`
}

type submitRequest struct {
	ProblemID uint   `json:"problem_id"`
	ContestID uint   `json:"contest_id"`
	Language  string `json:"language"`
	Code      string `json:"code"`
}

// CreateSubmission 校验并创建提交，随即投递评测任务。
func (h *Handler) CreateSubmission(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req submitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	sub, err := h.Judge.Submit(c.Request.Context(), service.SubmitInput{
		UserID:    u.ID,
		ProblemID: req.ProblemID,
		ContestID: req.ContestID,
		Language:  req.Language,
		Code:      req.Code,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSubmitTooFast):
			httpx.TooMany(c, httpx.CodeSubmitTooFast, err.Error())
		case errors.Is(err, service.ErrProblemNotVisible), errors.Is(err, service.ErrContestNotRunning),
			errors.Is(err, service.ErrNotRegistered), errors.Is(err, service.ErrCodeTooLong),
			errors.Is(err, service.ErrLanguageInvalid):
			httpx.BadRequest(c, err.Error())
		default:
			httpx.ServerError(c, err.Error())
		}
		return
	}
	httpx.OK(c, gin.H{"submission_id": sub.ID, "status": sub.Status})
}

// ListSubmissions 返回提交列表；普通用户只能查看自己的提交。
// ListSubmissions 需要登录后查看；登录后可浏览全站记录，mine=1 只看自己的。
func (h *Handler) ListSubmissions(c *gin.Context) {
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

// GetSubmission 需要登录；源码仅本人或管理员可见，其他人可看基本信息。
func (h *Handler) GetSubmission(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var sub model.Submission
	if err := h.DB.First(&sub, id).Error; err != nil {
		httpx.NotFound(c, "提交不存在")
		return
	}
	isAdmin := u.IsAdmin()
	isSelf := sub.UserID == u.ID
	// 比赛结束前，提交详情仅本人与管理员可见
	if sub.ContestID > 0 && !isAdmin && !isSelf && !h.contestRevealed(sub.ContestID) {
		httpx.Forbidden(c, "比赛进行中，暂不能查看提交详情")
		return
	}

	items := h.decorateSubmissions([]model.Submission{sub})
	canSeeCode := isSelf || isAdmin
	data := gin.H{
		"submission":       items[0],
		"code_visible":     canSeeCode,
		"testcase_results": sub.Results(),
	}
	if canSeeCode {
		data["code"] = sub.Code
	}
	if sub.Status == model.StatusCompileError {
		data["compile_error"] = sub.CompileError
	}
	if sub.JudgeMessage != "" {
		data["judge_message"] = sub.JudgeMessage
	}
	httpx.OK(c, data)
}

// GetSubmissionStatus 是评测页轮询用的轻量接口：只返回状态、分数、耗时和进度，
// 不带源码与测试点明细，轮询频率可以放得更高。
func (h *Handler) GetSubmissionStatus(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var sub model.Submission
	if err := h.DB.Select("id", "user_id", "contest_id", "status", "score", "time_ms",
		"memory_kb", "judged_at", "created_at").First(&sub, id).Error; err != nil {
		httpx.NotFound(c, "提交不存在")
		return
	}
	// 比赛结束前，别人的提交只对管理员可见
	if sub.ContestID > 0 && !u.IsAdmin() && sub.UserID != u.ID && !h.contestRevealed(sub.ContestID) {
		httpx.Forbidden(c, "比赛进行中，暂不能查看提交详情")
		return
	}

	data := gin.H{
		"id":         sub.ID,
		"status":     sub.Status,
		"score":      sub.Score,
		"time_ms":    sub.TimeMS,
		"memory_kb":  sub.MemoryKB,
		"judged_at":  sub.JudgedAt,
		"created_at": sub.CreatedAt,
	}
	if raw, err := h.RDB.Get(c.Request.Context(), service.JudgeProgressKey(sub.ID)).Result(); err == nil && raw != "" {
		var progress struct {
			Done      int   `json:"done"`
			Total     int   `json:"total"`
			UpdatedAt int64 `json:"updated_at"`
		}
		if json.Unmarshal([]byte(raw), &progress) == nil {
			data["progress"] = progress
		}
	}
	if sub.Status == model.StatusPending {
		// 同一个评测节点按提交顺序取任务，id 更小的 Pending 数量大致等于前面排队的人数
		var ahead int64
		_ = h.DB.Model(&model.Submission{}).
			Where("status = ? AND id < ?", model.StatusPending, sub.ID).Count(&ahead).Error
		data["pending_ahead"] = ahead
	}
	httpx.OK(c, data)
}

// querySubmissions 组装提交列表查询，contestID 为 0 时按参数中的 contest_id 过滤。
func (h *Handler) querySubmissions(c *gin.Context, u *model.User, contestID uint) ([]SubmissionItem, int64, int, int, error) {
	page, pageSize, offset := httpx.Pagination(c)

	query := h.DB.Model(&model.Submission{})
	if contestID > 0 {
		query = query.Where("contest_id = ?", contestID)
	} else if raw := strings.TrimSpace(c.Query("contest_id")); raw != "" {
		if raw == "0" {
			query = query.Where("contest_id = 0")
		} else {
			query = query.Where("contest_id = ?", httpx.QueryUint(c, "contest_id"))
		}
	}
	if pid := httpx.QueryUint(c, "problem_id"); pid > 0 {
		query = query.Where("problem_id = ?", pid)
	}
	if lang := strings.TrimSpace(c.Query("language")); lang != "" {
		query = query.Where("language = ?", lang)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}

	isAdmin := u != nil && u.IsAdmin()
	uid := httpx.QueryUint(c, "user_id")
	if isAdmin {
		if uid > 0 {
			query = query.Where("user_id = ?", uid)
		}
	} else if u != nil && c.Query("mine") == "1" {
		// 登录用户可筛选只看自己的提交
		query = query.Where("user_id = ?", u.ID)
	}
	if !isAdmin {
		// 比赛结束前的提交不公开，避免泄露比赛成绩
		query = query.Where("contest_id = 0 OR contest_id IN (?)",
			h.DB.Model(&model.Contest{}).Select("id").Where("end_at < ?", time.Now()))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}
	var subs []model.Submission
	if err := query.Order("id desc").Offset(offset).Limit(pageSize).Find(&subs).Error; err != nil {
		return nil, 0, 0, 0, err
	}
	return h.decorateSubmissions(subs), total, page, pageSize, nil
}

// contestRevealed 判断比赛是否已过封榜可公开查看他人提交。
func (h *Handler) contestRevealed(contestID uint) bool {
	var contest model.Contest
	if err := h.DB.First(&contest, contestID).Error; err != nil {
		return false
	}
	return !time.Now().Before(contest.EndAt)
}
