package handler

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 团队成员关系查询 -----------------------------------------------------------------

// teamMembership 返回用户在团队中的成员记录；非成员返回 nil。
func (h *Handler) teamMembership(userID, teamID uint) *model.TeamMember {
	if userID == 0 || teamID == 0 {
		return nil
	}
	var m model.TeamMember
	if err := h.DB.Where("team_id = ? AND user_id = ?", teamID, userID).First(&m).Error; err != nil {
		return nil
	}
	return &m
}

// isTeamMember 判断用户是否属于团队；管理员在内容访问上视为可见，但不参与成员判定。
func (h *Handler) isTeamMember(userID, teamID uint) bool {
	return h.teamMembership(userID, teamID) != nil
}

// myTeamIDs 返回用户加入的团队 ID 列表，用于过滤团队私有内容。
func (h *Handler) myTeamIDs(userID uint) []uint {
	if userID == 0 {
		return nil
	}
	var ids []uint
	_ = h.DB.Model(&model.TeamMember{}).Where("user_id = ?", userID).Pluck("team_id", &ids).Error
	return ids
}

// 团队 CRUD ---------------------------------------------------------------------

type teamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListTeams 返回团队列表（公开），附带当前用户的成员身份与申请状态。
func (h *Handler) ListTeams(c *gin.Context) {
	page, pageSize, offset := httpx.Pagination(c)

	query := h.DB.Model(&model.Team{})
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		httpx.ServerError(c, "查询团队失败")
		return
	}
	var teams []model.Team
	if err := query.Order("member_count desc, id asc").Offset(offset).Limit(pageSize).Find(&teams).Error; err != nil {
		httpx.ServerError(c, "查询团队失败")
		return
	}

	u := httpx.CurrentUser(c)
	roleMap := map[uint]string{}
	appliedMap := map[uint]bool{}
	if u != nil && len(teams) > 0 {
		ids := make([]uint, 0, len(teams))
		for _, t := range teams {
			ids = append(ids, t.ID)
		}
		var members []model.TeamMember
		_ = h.DB.Where("user_id = ? AND team_id IN ?", u.ID, ids).Find(&members).Error
		for _, m := range members {
			roleMap[m.TeamID] = m.Role
		}
		var apps []model.TeamApplication
		_ = h.DB.Where("user_id = ? AND team_id IN ? AND status = ?", u.ID, ids, model.TeamApplicationPending).Find(&apps).Error
		for _, a := range apps {
			appliedMap[a.TeamID] = true
		}
	}

	items := make([]gin.H, 0, len(teams))
	for _, t := range teams {
		items = append(items, gin.H{
			"id":           t.ID,
			"name":         t.Name,
			"description":  t.Description,
			"owner_id":     t.OwnerID,
			"member_count": t.MemberCount,
			"created_at":   t.CreatedAt,
			"my_role":      roleMap[t.ID],
			"applied":      appliedMap[t.ID],
		})
	}
	httpx.OK(c, httpx.List(total, page, pageSize, items))
}

// CreateTeam 创建团队，创建者自动成为队长。
func (h *Handler) CreateTeam(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req teamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if n := len([]rune(req.Name)); n < 2 || n > 32 {
		httpx.BadRequest(c, "团队名称长度需在 2 到 32 个字符之间")
		return
	}
	if len([]rune(req.Description)) > 255 {
		httpx.BadRequest(c, "团队简介不能超过 255 个字符")
		return
	}
	var exists int64
	if err := h.DB.Model(&model.Team{}).Where("name = ?", req.Name).Count(&exists).Error; err != nil {
		httpx.ServerError(c, "创建团队失败")
		return
	}
	if exists > 0 {
		httpx.Conflict(c, "团队名称已被使用")
		return
	}

	team := model.Team{Name: req.Name, Description: req.Description, OwnerID: u.ID, MemberCount: 1}
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&team).Error; err != nil {
			return err
		}
		return tx.Create(&model.TeamMember{
			TeamID:   team.ID,
			UserID:   u.ID,
			Role:     model.TeamRoleOwner,
			JoinedAt: time.Now(),
		}).Error
	})
	if err != nil {
		httpx.ServerError(c, "创建团队失败")
		return
	}
	logger.Businessf("团队创建成功 team=%s owner=%s", team.Name, u.Username)
	httpx.OK(c, gin.H{"team": team})
}

// GetTeam 返回团队详情；非成员也能看到基本信息与自己的申请状态。
func (h *Handler) GetTeam(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var team model.Team
	if err := h.DB.First(&team, id).Error; err != nil {
		httpx.NotFound(c, "团队不存在")
		return
	}

	u := httpx.CurrentUser(c)
	role := ""
	applied := false
	if u != nil {
		if m := h.teamMembership(u.ID, team.ID); m != nil {
			role = m.Role
		} else {
			var count int64
			_ = h.DB.Model(&model.TeamApplication{}).
				Where("team_id = ? AND user_id = ? AND status = ?", team.ID, u.ID, model.TeamApplicationPending).
				Count(&count).Error
			applied = count > 0
		}
	}

	httpx.OK(c, gin.H{
		"team":        team,
		"my_role":     role,
		"applied":     applied,
		"is_member":   role != "",
		"can_manage":  role == model.TeamRoleOwner,
	})
}

// UpdateTeam 队长修改团队信息。
func (h *Handler) UpdateTeam(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	team, ok := h.requireTeamOwner(c, id)
	if !ok {
		return
	}
	var req teamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if n := len([]rune(req.Name)); n < 2 || n > 32 {
		httpx.BadRequest(c, "团队名称长度需在 2 到 32 个字符之间")
		return
	}
	var exists int64
	if err := h.DB.Model(&model.Team{}).Where("name = ? AND id <> ?", req.Name, team.ID).Count(&exists).Error; err != nil {
		httpx.ServerError(c, "更新团队失败")
		return
	}
	if exists > 0 {
		httpx.Conflict(c, "团队名称已被使用")
		return
	}
	if err := h.DB.Model(team).Updates(map[string]any{"name": req.Name, "description": req.Description}).Error; err != nil {
		httpx.ServerError(c, "更新团队失败")
		return
	}
	httpx.OK(c, gin.H{"team": team})
}

// ListTeamMembers 返回团队成员列表（成员可见）。
func (h *Handler) ListTeamMembers(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.requireTeamMember(c, id); !ok {
		return
	}
	type row struct {
		UserID   uint      `json:"user_id"`
		Username string    `json:"username"`
		Nickname string    `json:"nickname"`
		Role     string    `json:"role"`
		JoinedAt time.Time `json:"joined_at"`
	}
	var rows []row
	if err := h.DB.Table("team_members AS m").
		Select("m.user_id, u.username, u.nickname, m.role, m.joined_at").
		Joins("JOIN users AS u ON u.id = m.user_id").
		Where("m.team_id = ?", id).
		Order("CASE m.role WHEN 'owner' THEN 0 ELSE 1 END, m.joined_at asc").
		Scan(&rows).Error; err != nil {
		httpx.ServerError(c, "查询成员失败")
		return
	}
	httpx.OK(c, gin.H{"items": rows})
}

// RemoveTeamMember 队长移除成员；成员可自行退出，队长不能退出。
func (h *Handler) RemoveTeamMember(c *gin.Context) {
	teamID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	targetID, ok := httpx.PathID(c, "uid")
	if !ok {
		return
	}
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	team, ok := h.requireTeamMember(c, teamID)
	if !ok {
		return
	}
	actor := h.teamMembership(u.ID, teamID)
	if actor == nil {
		return
	}
	if actor.Role != model.TeamRoleOwner && u.ID != targetID {
		httpx.Forbidden(c, "只有队长可以移除成员")
		return
	}
	if targetID == team.OwnerID {
		httpx.BadRequest(c, "队长不能退出团队")
		return
	}
	// 非管理员时，成员只能移除自己
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("team_id = ? AND user_id = ?", teamID, targetID).Delete(&model.TeamMember{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&model.Team{}).Where("id = ?", teamID).
			UpdateColumn("member_count", gorm.Expr("CASE WHEN member_count > 0 THEN member_count - 1 ELSE 0 END")).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.NotFound(c, "成员不存在")
		return
	}
	if err != nil {
		httpx.ServerError(c, "移除成员失败")
		return
	}
	httpx.OK(c, gin.H{})
}

// 申请与审批 ---------------------------------------------------------------------

type teamApplyRequest struct {
	Message string `json:"message"`
}

// ApplyTeam 提交加入申请；已是成员或有待审批申请时拒绝。
func (h *Handler) ApplyTeam(c *gin.Context) {
	teamID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var team model.Team
	if err := h.DB.First(&team, teamID).Error; err != nil {
		httpx.NotFound(c, "团队不存在")
		return
	}
	if h.isTeamMember(u.ID, teamID) {
		httpx.BadRequest(c, "你已经是该团队成员")
		return
	}
	var req teamApplyRequest
	_ = c.ShouldBindJSON(&req)
	req.Message = strings.TrimSpace(req.Message)
	if len([]rune(req.Message)) > 255 {
		httpx.BadRequest(c, "申请留言不能超过 255 个字符")
		return
	}

	var pending int64
	if err := h.DB.Model(&model.TeamApplication{}).
		Where("team_id = ? AND user_id = ? AND status = ?", teamID, u.ID, model.TeamApplicationPending).
		Count(&pending).Error; err != nil {
		httpx.ServerError(c, "提交申请失败")
		return
	}
	if pending > 0 {
		httpx.Conflict(c, "已提交过申请，请等待队长审批")
		return
	}

	app := model.TeamApplication{
		TeamID:  teamID,
		UserID:  u.ID,
		Message: req.Message,
		Status:  model.TeamApplicationPending,
	}
	if err := h.DB.Create(&app).Error; err != nil {
		httpx.ServerError(c, "提交申请失败")
		return
	}
	httpx.OK(c, gin.H{"application": app})
}

// ListTeamApplications 队长查看申请列表，默认只看待审批。
func (h *Handler) ListTeamApplications(c *gin.Context) {
	teamID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.requireTeamOwner(c, teamID); !ok {
		return
	}
	status := strings.TrimSpace(c.DefaultQuery("status", model.TeamApplicationPending))
	query := h.DB.Model(&model.TeamApplication{}).Where("team_id = ?", teamID)
	if status != "all" {
		query = query.Where("status = ?", status)
	}
	type row struct {
		ID        uint       `json:"id"`
		UserID    uint       `json:"user_id"`
		Username  string     `json:"username"`
		Nickname  string     `json:"nickname"`
		Message   string     `json:"message"`
		Status    string     `json:"status"`
		CreatedAt time.Time  `json:"created_at"`
		ReviewedAt *time.Time `json:"reviewed_at"`
	}
	var rows []row
	if err := query.
		Select("team_applications.id, team_applications.user_id, u.username, u.nickname, team_applications.message, team_applications.status, team_applications.created_at, team_applications.reviewed_at").
		Joins("JOIN users AS u ON u.id = team_applications.user_id").
		Order("team_applications.created_at asc").
		Scan(&rows).Error; err != nil {
		httpx.ServerError(c, "查询申请失败")
		return
	}
	httpx.OK(c, gin.H{"items": rows})
}

// ReviewTeamApplication 审批申请：action=approve 通过并加入团队，reject 拒绝。
func (h *Handler) ReviewTeamApplication(c *gin.Context) {
	teamID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	appID, ok := httpx.PathID(c, "aid")
	if !ok {
		return
	}
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	if _, ok := h.requireTeamOwner(c, teamID); !ok {
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))

	var app model.TeamApplication
	if err := h.DB.Where("id = ? AND team_id = ?", appID, teamID).First(&app).Error; err != nil {
		httpx.NotFound(c, "申请不存在")
		return
	}
	if app.Status != model.TeamApplicationPending {
		httpx.BadRequest(c, "该申请已处理")
		return
	}

	now := time.Now()
	switch req.Action {
	case "approve":
		err := h.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&app).Updates(map[string]any{
				"status":      model.TeamApplicationApproved,
				"reviewed_by": u.ID,
				"reviewed_at": now,
			}).Error; err != nil {
				return err
			}
			var exists int64
			if err := tx.Model(&model.TeamMember{}).Where("team_id = ? AND user_id = ?", teamID, app.UserID).Count(&exists).Error; err != nil {
				return err
			}
			if exists > 0 {
				return nil
			}
			if err := tx.Create(&model.TeamMember{
				TeamID:   teamID,
				UserID:   app.UserID,
				Role:     model.TeamRoleMember,
				JoinedAt: now,
			}).Error; err != nil {
				return err
			}
			return tx.Model(&model.Team{}).Where("id = ?", teamID).
				UpdateColumn("member_count", gorm.Expr("member_count + 1")).Error
		})
		if err != nil {
			httpx.ServerError(c, "审批失败")
			return
		}
	case "reject":
		if err := h.DB.Model(&app).Updates(map[string]any{
			"status":      model.TeamApplicationRejected,
			"reviewed_by": u.ID,
			"reviewed_at": now,
		}).Error; err != nil {
			httpx.ServerError(c, "审批失败")
			return
		}
	default:
		httpx.BadRequest(c, "action 只能是 approve 或 reject")
		return
	}
	httpx.OK(c, gin.H{})
}

// 权限辅助 ---------------------------------------------------------------------

// requireTeamMember 校验当前用户是团队成员，返回团队与成员记录。
func (h *Handler) requireTeamMember(c *gin.Context, teamID uint) (*model.Team, bool) {
	u := httpx.MustUser(c)
	if u == nil {
		return nil, false
	}
	var team model.Team
	if err := h.DB.First(&team, teamID).Error; err != nil {
		httpx.NotFound(c, "团队不存在")
		return nil, false
	}
	if h.teamMembership(u.ID, teamID) == nil && !u.IsAdmin() {
		httpx.Forbidden(c, "仅团队成员可访问")
		return nil, false
	}
	return &team, true
}

// requireTeamOwner 校验当前用户是队长。
func (h *Handler) requireTeamOwner(c *gin.Context, teamID uint) (*model.Team, bool) {
	u := httpx.MustUser(c)
	if u == nil {
		return nil, false
	}
	var team model.Team
	if err := h.DB.First(&team, teamID).Error; err != nil {
		httpx.NotFound(c, "团队不存在")
		return nil, false
	}
	m := h.teamMembership(u.ID, teamID)
	if m == nil || m.Role != model.TeamRoleOwner {
		httpx.Forbidden(c, "仅队长可执行该操作")
		return nil, false
	}
	return &team, true
}

// 团队题单 ---------------------------------------------------------------------

type teamSetRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ProblemIDs  []uint `json:"problem_ids"`
}

// ListTeamSets 返回团队题单（成员可见）。
func (h *Handler) ListTeamSets(c *gin.Context) {
	teamID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.requireTeamMember(c, teamID); !ok {
		return
	}
	var sets []model.TeamProblemSet
	if err := h.DB.Where("team_id = ?", teamID).Order("id desc").Find(&sets).Error; err != nil {
		httpx.ServerError(c, "查询题单失败")
		return
	}
	items := make([]gin.H, 0, len(sets))
	for _, s := range sets {
		ids := s.Problems()
		items = append(items, gin.H{
			"id":           s.ID,
			"title":        s.Title,
			"description":  s.Description,
			"problem_count": len(ids),
			"problem_ids":  ids,
			"created_by":   s.CreatedBy,
			"created_at":   s.CreatedAt,
		})
	}
	httpx.OK(c, gin.H{"items": items})
}

// CreateTeamSet 队长创建题单。
func (h *Handler) CreateTeamSet(c *gin.Context) {
	teamID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	if _, ok := h.requireTeamOwner(c, teamID); !ok {
		return
	}
	var req teamSetRequest
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
	idsJSON, err := json.Marshal(req.ProblemIDs)
	if err != nil {
		httpx.BadRequest(c, "题目列表格式错误")
		return
	}
	set := model.TeamProblemSet{
		TeamID:         teamID,
		Title:          req.Title,
		Description:    req.Description,
		ProblemIDsJSON: string(idsJSON),
		CreatedBy:      u.ID,
	}
	if err := h.DB.Create(&set).Error; err != nil {
		httpx.ServerError(c, "创建题单失败")
		return
	}
	httpx.OK(c, gin.H{"set": set})
}

// UpdateTeamSet 队长修改题单。
func (h *Handler) UpdateTeamSet(c *gin.Context) {
	teamID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	setID, ok := httpx.PathID(c, "sid")
	if !ok {
		return
	}
	if _, ok := h.requireTeamOwner(c, teamID); !ok {
		return
	}
	var req teamSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if n := len([]rune(req.Title)); n < 1 || n > 128 {
		httpx.BadRequest(c, "题单标题长度需在 1 到 128 个字符之间")
		return
	}
	idsJSON, err := json.Marshal(req.ProblemIDs)
	if err != nil {
		httpx.BadRequest(c, "题目列表格式错误")
		return
	}
	var set model.TeamProblemSet
	if err := h.DB.Where("id = ? AND team_id = ?", setID, teamID).First(&set).Error; err != nil {
		httpx.NotFound(c, "题单不存在")
		return
	}
	if err := h.DB.Model(&set).Updates(map[string]any{
		"title":         req.Title,
		"description":   req.Description,
		"problem_ids":   string(idsJSON),
	}).Error; err != nil {
		httpx.ServerError(c, "更新题单失败")
		return
	}
	httpx.OK(c, gin.H{"set": set})
}

// DeleteTeamSet 队长删除题单。
func (h *Handler) DeleteTeamSet(c *gin.Context) {
	teamID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	setID, ok := httpx.PathID(c, "sid")
	if !ok {
		return
	}
	if _, ok := h.requireTeamOwner(c, teamID); !ok {
		return
	}
	if err := h.DB.Where("id = ? AND team_id = ?", setID, teamID).Delete(&model.TeamProblemSet{}).Error; err != nil {
		httpx.ServerError(c, "删除题单失败")
		return
	}
	httpx.OK(c, gin.H{})
}

// 团队内容 ---------------------------------------------------------------------

// ListTeamProblems 返回团队专属题目（成员可见）。
func (h *Handler) ListTeamProblems(c *gin.Context) {
	teamID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.requireTeamMember(c, teamID); !ok {
		return
	}
	var problems []model.Problem
	if err := h.DB.Where("team_id = ?", teamID).Order("id asc").Find(&problems).Error; err != nil {
		httpx.ServerError(c, "查询团队题目失败")
		return
	}
	items := make([]gin.H, 0, len(problems))
	for i := range problems {
		p := &problems[i]
		items = append(items, gin.H{
			"id":              p.ID,
			"display_id":      p.DisplayID,
			"title":           p.Title,
			"difficulty":      p.Difficulty,
			"points":          p.Points,
			"submit_count":    p.SubmitCount,
			"accepted_count":  p.AcceptedCount,
			"testcase_count":  p.TestcaseCount,
			"created_at":      p.CreatedAt,
		})
	}
	httpx.OK(c, gin.H{"items": items})
}

// ListTeamContests 返回团队专属比赛（成员可见）。
func (h *Handler) ListTeamContests(c *gin.Context) {
	teamID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.requireTeamMember(c, teamID); !ok {
		return
	}
	var contests []model.Contest
	if err := h.DB.Where("team_id = ?", teamID).Order("start_at desc").Find(&contests).Error; err != nil {
		httpx.ServerError(c, "查询团队比赛失败")
		return
	}
	now := time.Now()
	items := make([]gin.H, 0, len(contests))
	for i := range contests {
		ct := &contests[i]
		items = append(items, gin.H{
			"id":          ct.ID,
			"title":       ct.Title,
			"start_at":    ct.StartAt,
			"end_at":      ct.EndAt,
			"status":      ct.Status(now),
			"register_required": ct.RegisterRequired,
		})
	}
	httpx.OK(c, gin.H{"items": items})
}
