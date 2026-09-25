package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
)

type contestItem struct {
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
	Registered       bool      `json:"registered"`
}

// ListContests 返回比赛列表，按开始时间倒序。
func (h *Handler) ListContests(c *gin.Context) {
	page, pageSize, offset := httpx.Pagination(c)
	now := time.Now()

	query := h.DB.Model(&model.Contest{})
	switch strings.TrimSpace(c.Query("status")) {
	case model.ContestStatusPending:
		query = query.Where("start_at > ?", now)
	case model.ContestStatusRunning:
		query = query.Where("start_at <= ? AND end_at >= ?", now, now)
	case model.ContestStatusEnded:
		query = query.Where("end_at < ?", now)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		httpx.ServerError(c, "查询比赛失败")
		return
	}
	var contests []model.Contest
	if err := query.Order("start_at desc").Offset(offset).Limit(pageSize).Find(&contests).Error; err != nil {
		httpx.ServerError(c, "查询比赛失败")
		return
	}

	joined := map[uint]bool{}
	if u := httpx.CurrentUser(c); u != nil && len(contests) > 0 {
		ids := make([]uint, 0, len(contests))
		for i := range contests {
			ids = append(ids, contests[i].ID)
		}
		var rows []model.ContestParticipant
		h.DB.Where("user_id = ? AND contest_id IN ?", u.ID, ids).Find(&rows)
		for _, r := range rows {
			joined[r.ContestID] = true
		}
	}

	items := make([]contestItem, 0, len(contests))
	for i := range contests {
		items = append(items, h.buildContestItem(&contests[i], now, joined[contests[i].ID]))
	}
	httpx.OK(c, httpx.List(total, page, pageSize, items))
}

// GetContest 返回比赛详情与题目列表。
func (h *Handler) GetContest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var contest model.Contest
	if err := h.DB.First(&contest, id).Error; err != nil {
		httpx.NotFound(c, "比赛不存在")
		return
	}
	now := time.Now()
	u := httpx.CurrentUser(c)

	registered := false
	if u != nil {
		var count int64
		h.DB.Model(&model.ContestParticipant{}).
			Where("contest_id = ? AND user_id = ?", contest.ID, u.ID).Count(&count)
		registered = count > 0
	}
	isManager := u != nil && u.CanManageProblems()

	type problemEntry struct {
		ProblemID     uint   `json:"problem_id"`
		DisplayID     string `json:"display_id"`
		Label         string `json:"label"`
		Title         string `json:"title"`
		Difficulty    string `json:"difficulty"`
		SubmitCount   int    `json:"submit_count"`
		AcceptedCount int    `json:"accepted_count"`
		MyScore       int    `json:"my_score"`
		FullScore     int    `json:"full_score"`
	}

	var cps []model.ContestProblem
	if err := h.DB.Where("contest_id = ?", contest.ID).Order("order_index asc, id asc").Find(&cps).Error; err != nil {
		httpx.ServerError(c, "查询比赛题目失败")
		return
	}
	entries := make([]problemEntry, 0, len(cps))
	if len(cps) > 0 {
		pids := make([]uint, 0, len(cps))
		for _, cp := range cps {
			pids = append(pids, cp.ProblemID)
		}
		var problems []model.Problem
		h.DB.Where("id IN ?", pids).Find(&problems)
		pmap := map[uint]model.Problem{}
		for i := range problems {
			pmap[problems[i].ID] = problems[i]
		}

		myBest := map[uint]int{}
		if u != nil {
			var subs []model.Submission
			h.DB.Where("contest_id = ? AND user_id = ?", contest.ID, u.ID).Find(&subs)
			for i := range subs {
				if subs[i].Score > myBest[subs[i].ProblemID] {
					myBest[subs[i].ProblemID] = subs[i].Score
				}
			}
		}
		for _, cp := range cps {
			p, ok := pmap[cp.ProblemID]
			if !ok {
				continue
			}
			entries = append(entries, problemEntry{
				ProblemID:     p.ID,
				DisplayID:     p.DisplayID,
				Label:         cp.Label,
				Title:         p.Title,
				Difficulty:    p.Difficulty,
				SubmitCount:   p.SubmitCount,
				AcceptedCount: p.AcceptedCount,
				MyScore:       myBest[p.ID],
				FullScore:     totalScore(&p),
			})
		}
	}

	status := contest.Status(now)
	httpx.OK(c, gin.H{
		"contest": gin.H{
			"id":                contest.ID,
			"title":             contest.Title,
			"description":       contest.Description,
			"start_at":          contest.StartAt,
			"end_at":            contest.EndAt,
			"freeze_minutes":    contest.FreezeMinutes,
			"freeze_at":         contest.FreezeAt(),
			"register_required": contest.RegisterRequired,
			"status":            status,
			"frozen":            contest.IsFrozen(now),
			"server_time":       now,
			"needs_password":    contest.Password != "",
		},
		"problems":   entries,
		"registered": registered,
		"can_submit": status == model.ContestStatusRunning && (!contest.RegisterRequired || registered) || isManager,
	})
}

type registerContestRequest struct {
	Password string `json:"password"`
}

// RegisterContest 报名比赛，按需校验比赛口令。
func (h *Handler) RegisterContest(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var contest model.Contest
	if err := h.DB.First(&contest, id).Error; err != nil {
		httpx.NotFound(c, "比赛不存在")
		return
	}
	var req registerContestRequest
	_ = c.ShouldBindJSON(&req)
	if contest.Password != "" && req.Password != contest.Password {
		httpx.BadRequest(c, "比赛口令不正确")
		return
	}
	var count int64
	h.DB.Model(&model.ContestParticipant{}).Where("contest_id = ? AND user_id = ?", contest.ID, u.ID).Count(&count)
	if count == 0 {
		if err := h.DB.Create(&model.ContestParticipant{
			ContestID: contest.ID, UserID: u.ID, RegisteredAt: time.Now(),
		}).Error; err != nil {
			httpx.ServerError(c, "报名失败")
			return
		}
	}
	httpx.OK(c, gin.H{"registered": true})
}

// ContestRank 返回比赛榜单；封榜期内仅管理员可见完整榜单。
func (h *Handler) ContestRank(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var contest model.Contest
	if err := h.DB.First(&contest, id).Error; err != nil {
		httpx.NotFound(c, "比赛不存在")
		return
	}
	u := httpx.CurrentUser(c)
	now := time.Now()
	revealAll := (u != nil && u.CanManageProblems()) || !now.Before(contest.EndAt)

	items, frozen, err := h.Rank.Compute(&contest, now, revealAll)
	if err != nil {
		httpx.ServerError(c, "榜单计算失败")
		return
	}
	httpx.OK(c, gin.H{
		"frozen":      frozen,
		"reveal_all":  revealAll,
		"server_time": now,
		"items":       items,
	})
}

// ContestRankCSV 导出榜单 CSV。
func (h *Handler) ContestRankCSV(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var contest model.Contest
	if err := h.DB.First(&contest, id).Error; err != nil {
		httpx.NotFound(c, "比赛不存在")
		return
	}
	u := httpx.CurrentUser(c)
	now := time.Now()
	revealAll := (u != nil && u.CanManageProblems()) || !now.Before(contest.EndAt)
	items, _, err := h.Rank.Compute(&contest, now, revealAll)
	if err != nil {
		httpx.ServerError(c, "榜单计算失败")
		return
	}

	var labels []string
	if len(items) > 0 {
		for _, cell := range items[0].Problems {
			labels = append(labels, cell.Label)
		}
	}
	var b strings.Builder
	b.WriteString("排名,用户名,昵称")
	for _, l := range labels {
		b.WriteString("," + l)
	}
	b.WriteString(",总分\n")
	for _, item := range items {
		row := []string{strconv.Itoa(item.Rank), item.Username, item.Nickname}
		for _, cell := range item.Problems {
			row = append(row, strconv.Itoa(cell.Score))
		}
		row = append(row, strconv.Itoa(item.TotalScore))
		b.WriteString(strings.Join(row, ",") + "\n")
	}

	filename := fmt.Sprintf("contest-%d-rank.csv", contest.ID)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", []byte("\ufeff"+b.String()))
}

// ContestSubmissions 返回比赛内提交；比赛结束前普通用户只能看到自己的提交。
func (h *Handler) ContestSubmissions(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var contest model.Contest
	if err := h.DB.First(&contest, id).Error; err != nil {
		httpx.NotFound(c, "比赛不存在")
		return
	}
	if u.CanManageProblems() || !time.Now().Before(contest.EndAt) {
		items, total, page, pageSize, err := h.querySubmissions(c, u, contest.ID)
		if err != nil {
			httpx.ServerError(c, "查询提交失败")
			return
		}
		httpx.OK(c, httpx.List(total, page, pageSize, items))
		return
	}

	page, pageSize, offset := httpx.Pagination(c)
	query := h.DB.Model(&model.Submission{}).Where("contest_id = ? AND user_id = ?", contest.ID, u.ID)
	if pid := httpx.QueryUint(c, "problem_id"); pid > 0 {
		query = query.Where("problem_id = ?", pid)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		httpx.ServerError(c, "查询提交失败")
		return
	}
	var subs []model.Submission
	if err := query.Order("id desc").Offset(offset).Limit(pageSize).Find(&subs).Error; err != nil {
		httpx.ServerError(c, "查询提交失败")
		return
	}
	httpx.OK(c, httpx.List(total, page, pageSize, h.decorateSubmissions(subs)))
}

func (h *Handler) buildContestItem(c *model.Contest, now time.Time, registered bool) contestItem {
	var problemCount, participantCount int64
	h.DB.Model(&model.ContestProblem{}).Where("contest_id = ?", c.ID).Count(&problemCount)
	h.DB.Model(&model.ContestParticipant{}).Where("contest_id = ?", c.ID).Count(&participantCount)
	return contestItem{
		ID:               c.ID,
		Title:            c.Title,
		Description:      c.Description,
		StartAt:          c.StartAt,
		EndAt:            c.EndAt,
		FreezeMinutes:    c.FreezeMinutes,
		RegisterRequired: c.RegisterRequired,
		NeedsPassword:    c.Password != "",
		Status:           c.Status(now),
		ProblemCount:     problemCount,
		ParticipantCount: participantCount,
		Registered:       registered,
	}
}
