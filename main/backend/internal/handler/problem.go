package handler

import (
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
)

type problemItem struct {
	ID             uint      `json:"id"`
	DisplayID      string    `json:"display_id"`
	Title          string    `json:"title"`
	Difficulty     string    `json:"difficulty"`
	Tags           []string  `json:"tags"`
	Source         string    `json:"source"`
	Points         int       `json:"points"`
	TimeLimitMS    int       `json:"time_limit_ms"`
	MemoryLimitMB  int       `json:"memory_limit_mb"`
	SubmitCount    int       `json:"submit_count"`
	AcceptedCount  int       `json:"accepted_count"`
	AcceptanceRate float64   `json:"acceptance_rate"`
	MyStatus       string    `json:"my_status"`
	CreatedAt      time.Time `json:"created_at"`
}

// ListProblems 返回公开题目列表，登录用户附带自己的通过状态。
func (h *Handler) ListProblems(c *gin.Context) {
	page, pageSize, offset := httpx.Pagination(c)

	query := h.DB.Model(&model.Problem{}).Where("visibility = ?", model.ProblemVisibilityPublic)
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("display_id = ? OR title LIKE ?", kw, like)
	}
	if diff := strings.TrimSpace(c.Query("difficulty")); diff != "" {
		query = query.Where("difficulty = ?", diff)
	}
	if tag := strings.TrimSpace(c.Query("tag")); tag != "" {
		query = query.Where("tags LIKE ?", "%\""+tag+"\"%")
	}
	if src := strings.TrimSpace(c.Query("source")); src != "" {
		query = query.Where("source LIKE ?", "%"+src+"%")
	}

	user := httpx.CurrentUser(c)
	statusFilter := strings.TrimSpace(c.Query("status"))
	var solvedSet, triedSet map[uint]bool
	if user != nil {
		solvedSet, triedSet = h.userProblemSets(user.ID)
		if statusFilter == "solved" {
			query = query.Where("id IN ?", keysOf(solvedSet))
		} else if statusFilter == "attempted" {
			query = query.Where("id IN ?", keysOf(triedSet))
		} else if statusFilter == "unsolved" {
			ids := keysOf(triedSet)
			if len(ids) > 0 {
				query = query.Where("id NOT IN ?", ids)
			}
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}
	var problems []model.Problem
	if err := query.Order("id asc").Offset(offset).Limit(pageSize).Find(&problems).Error; err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}

	items := make([]problemItem, 0, len(problems))
	for i := range problems {
		p := &problems[i]
		item := problemItem{
			ID:             p.ID,
			DisplayID:      p.DisplayID,
			Title:          p.Title,
			Difficulty:     p.Difficulty,
			Tags:           p.Tags(),
			Source:         p.Source,
			Points:         p.Points,
			TimeLimitMS:    p.TimeLimitMS,
			MemoryLimitMB:  p.MemoryLimitMB,
			SubmitCount:    p.SubmitCount,
			AcceptedCount:  p.AcceptedCount,
			AcceptanceRate: rate(p.AcceptedCount, p.SubmitCount),
			CreatedAt:      p.CreatedAt,
		}
		if solvedSet[p.ID] {
			item.MyStatus = "solved"
		} else if triedSet[p.ID] {
			item.MyStatus = "attempted"
		}
		items = append(items, item)
	}
	httpx.OK(c, httpx.List(total, page, pageSize, items))
}

// GetProblem 返回题目详情；比赛专用题在比赛开始后同样可访问。
func (h *Handler) GetProblem(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return
	}
	if !h.problemAccessible(c, &p) {
		httpx.NotFound(c, "题目不存在")
		return
	}

	item := problemItem{
		ID:             p.ID,
		DisplayID:      p.DisplayID,
		Title:          p.Title,
		Difficulty:     p.Difficulty,
		Tags:           p.Tags(),
		Source:         p.Source,
		Points:         p.Points,
		TimeLimitMS:    p.TimeLimitMS,
		MemoryLimitMB:  p.MemoryLimitMB,
		SubmitCount:    p.SubmitCount,
		AcceptedCount:  p.AcceptedCount,
		AcceptanceRate: rate(p.AcceptedCount, p.SubmitCount),
		CreatedAt:      p.CreatedAt,
	}
	if u := httpx.CurrentUser(c); u != nil {
		if h.userSolved(u.ID, p.ID) {
			item.MyStatus = "solved"
		} else if h.userTried(u.ID, p.ID) {
			item.MyStatus = "attempted"
		}
	}

	httpx.OK(c, gin.H{
		"problem": gin.H{
			"id":              p.ID,
			"display_id":      p.DisplayID,
			"title":           p.Title,
			"description":     p.Description,
			"input_format":    p.InputFormat,
			"output_format":   p.OutputFormat,
			"hint":            p.Hint,
			"samples":         p.Samples(),
			"time_limit_ms":   p.TimeLimitMS,
			"memory_limit_mb": p.MemoryLimitMB,
			"difficulty":      p.Difficulty,
			"tags":            p.Tags(),
			"source":          p.Source,
			"points":          p.Points,
			"submit_count":    p.SubmitCount,
			"accepted_count":  p.AcceptedCount,
			"acceptance_rate": item.AcceptanceRate,
			"contest_only":    p.ContestOnly,
			"testcase_count":  p.TestcaseCount,
			"total_score":     totalScore(&p),
			"created_at":      p.CreatedAt,
			"compare_mode":    p.CompareMode,
		},
		"my_status": item.MyStatus,
		"languages": model.SupportedLanguages(),
	})
}

// ListAnnouncements 返回可见公告。
func (h *Handler) ListAnnouncements(c *gin.Context) {
	var items []model.Announcement
	if err := h.DB.Where("visible = ?", true).Order("id desc").Limit(20).Find(&items).Error; err != nil {
		httpx.ServerError(c, "查询公告失败")
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

// ListLanguages 返回可选提交语言。
func (h *Handler) ListLanguages(c *gin.Context) {
	httpx.OK(c, gin.H{"items": model.SupportedLanguages()})
}

// SiteInfo 返回站点公开配置，供前端渲染登录方式与注册开关。
func (h *Handler) SiteInfo(c *gin.Context) {
	settings := h.Settings.Current()
	httpx.OK(c, gin.H{
		"app_name":                      settings.AppName,
		"register_enabled":              settings.RegisterEnabled && h.SMS.Enabled(),
		"register_invite_code_required": h.Cfg.RegisterInviteCode != "",
		"wechat_enabled":                h.WeChat.Enabled(),
		"sms_enabled":                   h.SMS.Enabled(),
		"languages":                     model.SupportedLanguages(),
	})
}

func (h *Handler) problemAccessible(c *gin.Context, p *model.Problem) bool {
	if p.Visibility == model.ProblemVisibilityPublic && !p.ContestOnly {
		return true
	}
	if u := httpx.CurrentUser(c); u != nil && u.IsAdmin() {
		return true
	}
	var count int64
	h.DB.Table("contest_problems").
		Joins("JOIN contests ON contests.id = contest_problems.contest_id").
		Where("contest_problems.problem_id = ? AND contests.start_at <= ?", p.ID, time.Now()).
		Count(&count)
	return count > 0
}

func (h *Handler) userProblemSets(userID uint) (solved, tried map[uint]bool) {
	solved = map[uint]bool{}
	tried = map[uint]bool{}
	var rows []model.Submission
	h.DB.Select("problem_id, status").Where("user_id = ?", userID).
		Group("problem_id, status").Find(&rows)
	for _, r := range rows {
		tried[r.ProblemID] = true
		if r.Status == model.StatusAccepted {
			solved[r.ProblemID] = true
		}
	}
	return solved, tried
}

func (h *Handler) userSolved(userID, problemID uint) bool {
	var count int64
	h.DB.Model(&model.Submission{}).
		Where("user_id = ? AND problem_id = ? AND status = ?", userID, problemID, model.StatusAccepted).
		Count(&count)
	return count > 0
}

func (h *Handler) userTried(userID, problemID uint) bool {
	var count int64
	h.DB.Model(&model.Submission{}).Where("user_id = ? AND problem_id = ?", userID, problemID).Count(&count)
	return count > 0
}

func rate(accepted, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(accepted) * 100 / float64(total)
}

func totalScore(p *model.Problem) int {
	total := 0
	for _, s := range p.Scores() {
		total += s
	}
	return total
}

func keysOf(m map[uint]bool) []uint {
	out := make([]uint, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	if len(out) == 0 {
		out = append(out, 0)
	}
	return out
}
