package handler

import (
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
)

// 题单 -------------------------------------------------------------------------

type problemSetRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ProblemIDs  []uint `json:"problem_ids"`
	// TeamID 归属团队；0 或空表示个人题单
	TeamID uint `json:"team_id"`
}

type problemSetItem struct {
	ID           uint      `json:"id"`
	Kind         string    `json:"kind"`
	TeamID       uint      `json:"team_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	ProblemCount int       `json:"problem_count"`
	CreatedBy    uint      `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ListMyProblemSets 只返回当前用户的个人题单；团队题单请在团队页面查看。
func (h *Handler) ListMyProblemSets(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var personal []model.UserProblemSet
	if err := h.DB.Where("owner_id = ?", u.ID).Order("id desc").Find(&personal).Error; err != nil {
		httpx.ServerError(c, "查询题单失败")
		return
	}
	items := make([]problemSetItem, 0, len(personal))
	for i := range personal {
		s := &personal[i]
		items = append(items, problemSetItem{
			ID:           s.ID,
			Kind:         model.ProblemKindPersonal,
			Title:        s.Title,
			Description:  s.Description,
			ProblemCount: len(s.Problems()),
			CreatedBy:    s.OwnerID,
			CreatedAt:    s.CreatedAt,
			UpdatedAt:    s.UpdatedAt,
		})
	}
	httpx.OK(c, gin.H{"items": items})
}

// problemSetProblems 按题单内的题目 ID 返回可见题目，附带当前用户的通过状态。
func (h *Handler) problemSetProblems(c *gin.Context, ids []uint) []gin.H {
	items := []gin.H{}
	if len(ids) == 0 {
		return items
	}
	var problems []model.Problem
	if err := h.DB.Where("id IN ?", ids).Find(&problems).Error; err != nil {
		return items
	}
	byID := make(map[uint]*model.Problem, len(problems))
	for i := range problems {
		byID[problems[i].ID] = &problems[i]
	}
	u := httpx.CurrentUser(c)
	var solved, tried map[uint]bool
	if u != nil {
		solved, tried = h.userProblemSets(u.ID)
	}
	for _, id := range ids {
		p := byID[id]
		if p == nil {
			continue
		}
		if !h.canReadProblem(u, p) {
			continue
		}
		status := ""
		if solved[id] {
			status = "solved"
		} else if tried[id] {
			status = "attempted"
		}
		items = append(items, gin.H{
			"id":         p.ID,
			"display_id": p.DisplayID,
			"title":      p.Title,
			"difficulty": p.Difficulty,
			"tags":       p.Tags(),
			"points":     p.Points,
			"kind":       p.Kind(),
			"access":     p.Access,
			"my_status":  status,
			"created_at": p.CreatedAt,
		})
	}
	return items
}

// normalizeProblemIDs 去重并校验题目全部存在，返回去重后的 ID 列表。
func (h *Handler) normalizeProblemIDs(ids []uint) ([]uint, bool) {
	out := make([]uint, 0, len(ids))
	seen := map[uint]bool{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return out, true
	}
	if len(out) > 500 {
		return nil, false
	}
	var count int64
	if err := h.DB.Model(&model.Problem{}).Where("id IN ?", out).Count(&count).Error; err != nil {
		return nil, false
	}
	return out, int(count) == len(out)
}

// CreateMyProblemSet 创建个人题单或团队题单（团队题单仅队长可建）。
func (h *Handler) CreateMyProblemSet(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req problemSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if n := len([]rune(req.Title)); n < 1 || n > 128 {
		httpx.BadRequest(c, "题单标题长度需在 1 到 128 个字符之间")
		return
	}
	if len([]rune(req.Description)) > 255 {
		httpx.BadRequest(c, "题单说明不能超过 255 个字符")
		return
	}
	ids, ok := h.normalizeProblemIDs(req.ProblemIDs)
	if !ok {
		httpx.BadRequest(c, "题目列表不合法或包含不存在的题目")
		return
	}

	if req.TeamID > 0 {
		if _, ok := h.requireTeamOwner(c, req.TeamID); !ok {
			return
		}
		set := model.TeamProblemSet{
			TeamID:      req.TeamID,
			Title:       req.Title,
			Description: req.Description,
			CreatedBy:   u.ID,
		}
		set.SetProblems(ids)
		if err := h.DB.Create(&set).Error; err != nil {
			httpx.ServerError(c, "创建题单失败")
			return
		}
		httpx.OK(c, gin.H{"set": gin.H{"id": set.ID, "kind": model.ProblemKindTeam}})
		return
	}

	set := model.UserProblemSet{
		OwnerID:     u.ID,
		Title:       req.Title,
		Description: req.Description,
	}
	set.SetProblems(ids)
	if err := h.DB.Create(&set).Error; err != nil {
		httpx.ServerError(c, "创建题单失败")
		return
	}
	httpx.OK(c, gin.H{"set": gin.H{"id": set.ID, "kind": model.ProblemKindPersonal}})
}

// GetMyProblemSet 返回个人题单详情与题目列表。
func (h *Handler) GetMyProblemSet(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var set model.UserProblemSet
	if err := h.DB.Where("id = ? AND owner_id = ?", id, u.ID).First(&set).Error; err != nil {
		httpx.NotFound(c, "题单不存在")
		return
	}
	ids := set.Problems()
	httpx.OK(c, gin.H{
		"set": gin.H{
			"id":          set.ID,
			"kind":        model.ProblemKindPersonal,
			"title":       set.Title,
			"description": set.Description,
			"created_at":  set.CreatedAt,
			"updated_at":  set.UpdatedAt,
		},
		"problems": h.problemSetProblems(c, ids),
	})
}

// UpdateMyProblemSet 修改个人题单。
func (h *Handler) UpdateMyProblemSet(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var set model.UserProblemSet
	if err := h.DB.Where("id = ? AND owner_id = ?", id, u.ID).First(&set).Error; err != nil {
		httpx.NotFound(c, "题单不存在")
		return
	}
	var req problemSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if n := len([]rune(req.Title)); n < 1 || n > 128 {
		httpx.BadRequest(c, "题单标题长度需在 1 到 128 个字符之间")
		return
	}
	if len([]rune(req.Description)) > 255 {
		httpx.BadRequest(c, "题单说明不能超过 255 个字符")
		return
	}
	ids, ok := h.normalizeProblemIDs(req.ProblemIDs)
	if !ok {
		httpx.BadRequest(c, "题目列表不合法或包含不存在的题目")
		return
	}
	set.Title = req.Title
	set.Description = req.Description
	set.SetProblems(ids)
	if err := h.DB.Save(&set).Error; err != nil {
		httpx.ServerError(c, "更新题单失败")
		return
	}
	httpx.OK(c, gin.H{"set": gin.H{"id": set.ID}})
}

// DeleteMyProblemSet 删除个人题单。
func (h *Handler) DeleteMyProblemSet(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := h.DB.Where("id = ? AND owner_id = ?", id, u.ID).Delete(&model.UserProblemSet{}).Error; err != nil {
		httpx.ServerError(c, "删除题单失败")
		return
	}
	httpx.OK(c, gin.H{})
}

// GetTeamSet 返回团队题单详情与题目列表（成员可见）。
func (h *Handler) GetTeamSet(c *gin.Context) {
	teamID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	setID, ok := httpx.PathID(c, "sid")
	if !ok {
		return
	}
	if _, ok := h.requireTeamMember(c, teamID); !ok {
		return
	}
	var set model.TeamProblemSet
	if err := h.DB.Where("id = ? AND team_id = ?", setID, teamID).First(&set).Error; err != nil {
		httpx.NotFound(c, "题单不存在")
		return
	}
	ids := set.Problems()
	httpx.OK(c, gin.H{
		"set": gin.H{
			"id":          set.ID,
			"kind":        model.ProblemKindTeam,
			"team_id":     set.TeamID,
			"title":       set.Title,
			"description": set.Description,
			"created_by":  set.CreatedBy,
			"created_at":  set.CreatedAt,
			"updated_at":  set.UpdatedAt,
		},
		"problems": h.problemSetProblems(c, ids),
	})
}
