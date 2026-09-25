package handler

import (
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminContestItem struct {
	ID               uint      `json:"id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	StartAt          time.Time `json:"start_at"`
	EndAt            time.Time `json:"end_at"`
	FreezeMinutes    int       `json:"freeze_minutes"`
	RegisterRequired bool      `json:"register_required"`
	NeedsPassword    bool      `json:"needs_password"`
	Status           string    `json:"status"`
	ProblemCount     int64     `json:"problem_count"`
	ParticipantCount int64     `json:"participant_count"`
}

// contestCountRow 承载按比赛分组的计数结果。
type contestCountRow struct {
	ContestID uint
	Total     int64
}

// AdminListContests 返回比赛列表。
func (h *Handler) AdminListContests(c *gin.Context) {
	page, pageSize, offset := httpx.Pagination(c)
	now := time.Now()

	query := h.DB.Model(&model.Contest{})
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		query = query.Where("title LIKE ?", "%"+kw+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		httpx.ServerError(c, "查询比赛失败")
		return
	}
	var contests []model.Contest
	if err := query.Order("start_at desc, id desc").Offset(offset).Limit(pageSize).Find(&contests).Error; err != nil {
		httpx.ServerError(c, "查询比赛失败")
		return
	}

	problemCounts := map[uint]int64{}
	participantCounts := map[uint]int64{}
	if len(contests) > 0 {
		ids := make([]uint, 0, len(contests))
		for i := range contests {
			ids = append(ids, contests[i].ID)
		}
		var countRows []contestCountRow
		if err := h.DB.Model(&model.ContestProblem{}).
			Select("contest_id, count(*) as total").Where("contest_id IN ?", ids).
			Group("contest_id").Scan(&countRows).Error; err != nil {
			httpx.ServerError(c, "查询比赛失败")
			return
		}
		for _, r := range countRows {
			problemCounts[r.ContestID] = r.Total
		}
		countRows = nil
		if err := h.DB.Model(&model.ContestParticipant{}).
			Select("contest_id, count(*) as total").Where("contest_id IN ?", ids).
			Group("contest_id").Scan(&countRows).Error; err != nil {
			httpx.ServerError(c, "查询比赛失败")
			return
		}
		for _, r := range countRows {
			participantCounts[r.ContestID] = r.Total
		}
	}

	items := make([]adminContestItem, 0, len(contests))
	for i := range contests {
		c0 := &contests[i]
		items = append(items, adminContestItem{
			ID:               c0.ID,
			Title:            c0.Title,
			Description:      c0.Description,
			StartAt:          c0.StartAt,
			EndAt:            c0.EndAt,
			FreezeMinutes:    c0.FreezeMinutes,
			RegisterRequired: c0.RegisterRequired,
			NeedsPassword:    c0.Password != "",
			Status:           c0.Status(now),
			ProblemCount:     problemCounts[c0.ID],
			ParticipantCount: participantCounts[c0.ID],
		})
	}
	httpx.OK(c, httpx.List(total, page, pageSize, items))
}

type adminContestProblemEntry struct {
	ProblemID  uint   `json:"problem_id"`
	DisplayID  string `json:"display_id"`
	Label      string `json:"label"`
	OrderIndex int    `json:"order_index"`
	Title      string `json:"title"`
}

// AdminGetContest 返回比赛详情与其题目列表。
func (h *Handler) AdminGetContest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var contest model.Contest
	if err := h.DB.First(&contest, id).Error; err != nil {
		httpx.NotFound(c, "比赛不存在")
		return
	}
	entries, err := h.contestProblemEntries(contest.ID)
	if err != nil {
		httpx.ServerError(c, "查询比赛题目失败")
		return
	}
	httpx.OK(c, gin.H{
		"contest": gin.H{
			"id":                contest.ID,
			"title":             contest.Title,
			"description":       contest.Description,
			"start_at":          contest.StartAt,
			"end_at":            contest.EndAt,
			"freeze_minutes":    contest.FreezeMinutes,
			"register_required": contest.RegisterRequired,
			"needs_password":    contest.Password != "",
			"status":            contest.Status(time.Now()),
		},
		"problems": entries,
	})
}

func (h *Handler) contestProblemEntries(contestID uint) ([]adminContestProblemEntry, error) {
	var cps []model.ContestProblem
	if err := h.DB.Where("contest_id = ?", contestID).Order("order_index asc, id asc").Find(&cps).Error; err != nil {
		return nil, err
	}
	entries := make([]adminContestProblemEntry, 0, len(cps))
	if len(cps) == 0 {
		return entries, nil
	}
	pids := make([]uint, 0, len(cps))
	for _, cp := range cps {
		pids = append(pids, cp.ProblemID)
	}
	var problems []model.Problem
	if err := h.DB.Where("id IN ?", pids).Find(&problems).Error; err != nil {
		return nil, err
	}
	pmap := map[uint]model.Problem{}
	for i := range problems {
		pmap[problems[i].ID] = problems[i]
	}
	for _, cp := range cps {
		e := adminContestProblemEntry{ProblemID: cp.ProblemID, Label: cp.Label, OrderIndex: cp.OrderIndex}
		if p, ok := pmap[cp.ProblemID]; ok {
			e.DisplayID = p.DisplayID
			e.Title = p.Title
		}
		entries = append(entries, e)
	}
	return entries, nil
}

type adminContestRequest struct {
	Title            *string `json:"title"`
	Description      *string `json:"description"`
	StartAt          *string `json:"start_at"`
	EndAt            *string `json:"end_at"`
	FreezeMinutes    *int    `json:"freeze_minutes"`
	Password         *string `json:"password"`
	RegisterRequired *bool   `json:"register_required"`
}

// parseTime 解析 RFC3339 或 "2006-01-02 15:04:05" 两种时间格式。
func parseTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	return time.ParseInLocation("2006-01-02 15:04:05", raw, time.Local)
}

// AdminCreateContest 创建比赛。
func (h *Handler) AdminCreateContest(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req adminContestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}

	title := ""
	if req.Title != nil {
		title = strings.TrimSpace(*req.Title)
	}
	if title == "" {
		httpx.BadRequest(c, "比赛标题不能为空")
		return
	}
	if req.StartAt == nil || req.EndAt == nil {
		httpx.BadRequest(c, "必须指定比赛起止时间")
		return
	}
	startAt, err := parseTime(*req.StartAt)
	if err != nil {
		httpx.BadRequest(c, "开始时间格式错误")
		return
	}
	endAt, err := parseTime(*req.EndAt)
	if err != nil {
		httpx.BadRequest(c, "结束时间格式错误")
		return
	}
	if !endAt.After(startAt) {
		httpx.BadRequest(c, "结束时间必须晚于开始时间")
		return
	}
	freeze := 0
	if req.FreezeMinutes != nil {
		if *req.FreezeMinutes < 0 {
			httpx.BadRequest(c, "封榜时长不能为负")
			return
		}
		freeze = *req.FreezeMinutes
	}

	contest := model.Contest{
		Title:         title,
		StartAt:       startAt,
		EndAt:         endAt,
		FreezeMinutes: freeze,
		CreatedBy:     u.ID,
	}
	if req.Description != nil {
		contest.Description = *req.Description
	}
	if req.Password != nil {
		contest.Password = strings.TrimSpace(*req.Password)
	}
	if req.RegisterRequired != nil {
		contest.RegisterRequired = *req.RegisterRequired
	}

	if err := h.DB.Create(&contest).Error; err != nil {
		httpx.ServerError(c, "创建比赛失败")
		return
	}
	httpx.OK(c, gin.H{"contest": gin.H{"id": contest.ID}})
}

// AdminUpdateContest 更新比赛，仅更新请求中出现的字段。
func (h *Handler) AdminUpdateContest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var contest model.Contest
	if err := h.DB.First(&contest, id).Error; err != nil {
		httpx.NotFound(c, "比赛不存在")
		return
	}
	var req adminContestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}

	updates := map[string]any{}
	startAt, endAt := contest.StartAt, contest.EndAt
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			httpx.BadRequest(c, "比赛标题不能为空")
			return
		}
		updates["title"] = title
	}
	if req.StartAt != nil {
		t, err := parseTime(*req.StartAt)
		if err != nil {
			httpx.BadRequest(c, "开始时间格式错误")
			return
		}
		startAt = t
		updates["start_at"] = t
	}
	if req.EndAt != nil {
		t, err := parseTime(*req.EndAt)
		if err != nil {
			httpx.BadRequest(c, "结束时间格式错误")
			return
		}
		endAt = t
		updates["end_at"] = t
	}
	if (req.StartAt != nil || req.EndAt != nil) && !endAt.After(startAt) {
		httpx.BadRequest(c, "结束时间必须晚于开始时间")
		return
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.FreezeMinutes != nil {
		if *req.FreezeMinutes < 0 {
			httpx.BadRequest(c, "封榜时长不能为负")
			return
		}
		updates["freeze_minutes"] = *req.FreezeMinutes
	}
	if req.Password != nil {
		updates["password"] = strings.TrimSpace(*req.Password)
	}
	if req.RegisterRequired != nil {
		updates["register_required"] = *req.RegisterRequired
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&model.Contest{}).Where("id = ?", contest.ID).Updates(updates).Error; err != nil {
			httpx.ServerError(c, "更新比赛失败")
			return
		}
	}
	httpx.OK(c, gin.H{"contest": gin.H{"id": contest.ID}})
}

// AdminDeleteContest 删除比赛，并清理其题目关联与报名记录。
func (h *Handler) AdminDeleteContest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var contest model.Contest
	if err := h.DB.First(&contest, id).Error; err != nil {
		httpx.NotFound(c, "比赛不存在")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("contest_id = ?", contest.ID).Delete(&model.ContestProblem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("contest_id = ?", contest.ID).Delete(&model.ContestParticipant{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Contest{}, contest.ID).Error
	}); err != nil {
		httpx.ServerError(c, "删除比赛失败")
		return
	}
	httpx.OK(c, gin.H{})
}

type adminContestProblemsRequest struct {
	Problems []struct {
		ProblemID  uint   `json:"problem_id"`
		Label      string `json:"label"`
		OrderIndex int    `json:"order_index"`
	} `json:"problems"`
}

// AdminSetContestProblems 整体替换比赛的题目列表。
func (h *Handler) AdminSetContestProblems(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var contest model.Contest
	if err := h.DB.First(&contest, id).Error; err != nil {
		httpx.NotFound(c, "比赛不存在")
		return
	}
	var req adminContestProblemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}

	rows := make([]model.ContestProblem, 0, len(req.Problems))
	for i, item := range req.Problems {
		if item.ProblemID == 0 {
			httpx.BadRequest(c, "题目 ID 非法")
			return
		}
		label := strings.TrimSpace(item.Label)
		if label == "" {
			label = contestLabel(i)
		}
		orderIndex := item.OrderIndex
		if orderIndex == 0 {
			orderIndex = i + 1
		}
		rows = append(rows, model.ContestProblem{
			ContestID:  contest.ID,
			ProblemID:  item.ProblemID,
			Label:      label,
			OrderIndex: orderIndex,
		})
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("contest_id = ?", contest.ID).Delete(&model.ContestProblem{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	}); err != nil {
		httpx.ServerError(c, "保存比赛题目失败")
		return
	}
	httpx.OK(c, gin.H{})
}

// contestLabel 按下标生成 A、B…Z、AA、AB… 形式的题号。
func contestLabel(i int) string {
	label := ""
	for i >= 0 {
		label = string(rune('A'+i%26)) + label
		i = i/26 - 1
	}
	return label
}
