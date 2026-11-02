package handler

import (
	"strconv"
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
)

// 用户的团队题目与个人题目 -------------------------------------------------------

type myProblemItem struct {
	ID            uint      `json:"id"`
	DisplayID     string    `json:"display_id"`
	BankID        uint      `json:"bank_id"`
	BankName      string    `json:"bank_name"`
	Kind          string    `json:"kind"`
	TeamID        uint      `json:"team_id"`
	OwnerID       uint      `json:"owner_id"`
	Access        int       `json:"access"`
	Title         string    `json:"title"`
	Difficulty    string    `json:"difficulty"`
	Tags          []string  `json:"tags"`
	Points        int       `json:"points"`
	TimeLimitMS   int       `json:"time_limit_ms"`
	MemoryLimitMB int       `json:"memory_limit_mb"`
	TestcaseCount int       `json:"testcase_count"`
	TotalScore    int       `json:"total_score"`
	SubmitCount   int       `json:"submit_count"`
	AcceptedCount int       `json:"accepted_count"`
	AuthorID      uint      `json:"author_id"`
	AuthorName    string    `json:"author_name"`
	Visibility    int       `json:"visibility"`
	CreatedAt     time.Time `json:"created_at"`
}

func toMyProblemItem(p *model.Problem, bankName, authorName string) myProblemItem {
	return myProblemItem{
		ID:            p.ID,
		DisplayID:     p.DisplayID,
		BankID:        p.BankID,
		BankName:      bankName,
		Kind:          p.Kind(),
		TeamID:        p.TeamID,
		OwnerID:       p.OwnerID,
		Access:        p.Access,
		Title:         p.Title,
		Difficulty:    p.Difficulty,
		Tags:          p.Tags(),
		Points:        p.Points,
		TimeLimitMS:   p.TimeLimitMS,
		MemoryLimitMB: p.MemoryLimitMB,
		TestcaseCount: p.TestcaseCount,
		TotalScore:    totalScore(p),
		SubmitCount:   p.SubmitCount,
		AcceptedCount: p.AcceptedCount,
		AuthorID:      p.CreatedBy,
		AuthorName:    authorName,
		Visibility:    p.Visibility,
		CreatedAt:     p.CreatedAt,
	}
}

// canReadProblem 判断用户能否查看该题：公共题目所有人可看，受限题目仅团队成员或创建者。
func (h *Handler) canReadProblem(u *model.User, p *model.Problem) bool {
	if p.Access == model.ProblemAccessPublic {
		return true
	}
	if u == nil {
		return false
	}
	if u.IsAdmin() {
		return true
	}
	if p.IsTeamProblem() {
		return h.isTeamMember(u.ID, p.TeamID)
	}
	if p.IsPersonalProblem() {
		return p.OwnerID == u.ID
	}
	return true
}

// ListMyProblems 返回当前用户可管理的题目：自己创建的个人题目与所在团队的团队题目。
func (h *Handler) ListMyProblems(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	page, pageSize, offset := httpx.Pagination(c)

	teamIDs := h.myTeamIDs(u.ID)
	if len(teamIDs) == 0 {
		teamIDs = []uint{0}
	}
	query := h.DB.Model(&model.Problem{}).Where("owner_id = ? OR team_id IN ?", u.ID, teamIDs)

	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("display_id = ? OR title LIKE ?", kw, like)
	}
	if bankID := strings.TrimSpace(c.Query("bank_id")); bankID != "" {
		id, err := strconv.ParseUint(bankID, 10, 64)
		if err != nil {
			httpx.BadRequest(c, "题库参数非法")
			return
		}
		query = query.Where("bank_id = ?", id)
	}
	switch strings.TrimSpace(c.Query("kind")) {
	case model.ProblemKindTeam:
		query = query.Where("team_id > 0")
	case model.ProblemKindPersonal:
		query = query.Where("team_id = 0 AND owner_id > 0")
	}
	if raw := strings.TrimSpace(c.Query("team_id")); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			httpx.BadRequest(c, "团队参数非法")
			return
		}
		query = query.Where("team_id = ?", id)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}
	var problems []model.Problem
	if err := query.Order("bank_id asc, display_id asc").Offset(offset).Limit(pageSize).Find(&problems).Error; err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}
	bankNames, err := h.problemBankNames()
	if err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}

	items := make([]myProblemItem, 0, len(problems))
	authorNames, err := h.problemAuthorNames(problems)
	if err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}
	for i := range problems {
		p := &problems[i]
		items = append(items, toMyProblemItem(p, bankNames[p.BankID], authorNames[p.CreatedBy]))
	}
	httpx.OK(c, httpx.List(total, page, pageSize, items))
}

// loadManageableProblem 读取题目并校验当前用户有管理权限。
func (h *Handler) loadManageableProblem(c *gin.Context) (*model.Problem, bool) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return nil, false
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return nil, false
	}
	u := httpx.MustUser(c)
	if u == nil {
		return nil, false
	}
	if !h.canManageProblem(u, &p) {
		httpx.Forbidden(c, "仅团队队长或题目创建者可执行该操作")
		return nil, false
	}
	return &p, true
}

// GetMyProblem 返回题目详情，供创建者或队长编辑题目与管理测试数据。
func (h *Handler) GetMyProblem(c *gin.Context) {
	p, ok := h.loadManageableProblem(c)
	if !ok {
		return
	}
	bankNames, err := h.problemBankNames()
	if err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}
	detail := adminProblemDetail(p)
	detail["bank_name"] = bankNames[p.BankID]
	httpx.OK(c, gin.H{"problem": detail})
}

type myProblemRequest struct {
	Title         *string         `json:"title"`
	Description   *string         `json:"description"`
	InputFormat   *string         `json:"input_format"`
	OutputFormat  *string         `json:"output_format"`
	Hint          *string         `json:"hint"`
	Samples       *[]model.Sample `json:"samples"`
	TimeLimitMS   *int            `json:"time_limit_ms"`
	MemoryLimitMB *int            `json:"memory_limit_mb"`
	Difficulty    *string         `json:"difficulty"`
	Tags          *[]string       `json:"tags"`
	Source        *string         `json:"source"`
	Points        *int            `json:"points"`
	CompareMode   *string         `json:"compare_mode"`
	FloatEps      *float64        `json:"float_eps"`
	// TeamID 归属团队；0 或空表示个人题目
	TeamID *uint `json:"team_id"`
	// Access 访问权限：1 公开，2 仅团队成员或本人
	Access *int `json:"access"`
}

// CreateMyProblem 创建个人题目或团队题目，题号由所属题库自动分配。
func (h *Handler) CreateMyProblem(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req myProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}

	title := ""
	if req.Title != nil {
		title = strings.TrimSpace(*req.Title)
	}
	if title == "" {
		httpx.BadRequest(c, "题目标题不能为空")
		return
	}
	if n := len([]rune(title)); n > 128 {
		httpx.BadRequest(c, "题目标题不能超过 128 个字符")
		return
	}

	timeLimit := 1000
	if req.TimeLimitMS != nil {
		if *req.TimeLimitMS < 100 || *req.TimeLimitMS > 30000 {
			httpx.BadRequest(c, "时间限制需在 100 到 30000 毫秒之间")
			return
		}
		timeLimit = *req.TimeLimitMS
	}
	memoryLimit := 256
	if req.MemoryLimitMB != nil {
		if *req.MemoryLimitMB < 16 || *req.MemoryLimitMB > 2048 {
			httpx.BadRequest(c, "内存限制需在 16 到 2048 MB 之间")
			return
		}
		memoryLimit = *req.MemoryLimitMB
	}
	compareMode := model.CompareModeDefault
	if req.CompareMode != nil {
		cm := strings.TrimSpace(*req.CompareMode)
		if cm != model.CompareModeDefault && cm != model.CompareModeExact && cm != model.CompareModeFloat {
			httpx.BadRequest(c, "比对模式取值非法")
			return
		}
		compareMode = cm
	}
	floatEps := 1e-6
	if req.FloatEps != nil {
		floatEps = *req.FloatEps
	}
	difficulty := ""
	if req.Difficulty != nil {
		difficulty = strings.TrimSpace(*req.Difficulty)
	}
	points := model.DefaultPoints(difficulty)
	if req.Points != nil {
		if *req.Points < 0 || *req.Points > 100000 {
			httpx.BadRequest(c, "题目积分需在 0 到 100000 之间")
			return
		}
		points = *req.Points
	}

	// 归属：团队题目要求操作者是队长，未归属团队的题目归为个人题目
	teamID := uint(0)
	if req.TeamID != nil && *req.TeamID > 0 {
		if _, ok := h.requireTeamOwner(c, *req.TeamID); !ok {
			return
		}
		teamID = *req.TeamID
	}
	ownerID := uint(0)
	if teamID == 0 {
		ownerID = u.ID
	}
	access := defaultProblemAccess(teamID, ownerID)
	if req.Access != nil {
		if !validProblemAccessFor(teamID, ownerID, *req.Access) {
			httpx.BadRequest(c, "访问权限取值非法")
			return
		}
		access = *req.Access
	}
	bank, err := h.problemBankForOwner(teamID, ownerID)
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	displayID, err := h.allocateProblemDisplayID(bank)
	if err != nil {
		httpx.ServerError(c, "生成题号失败")
		return
	}

	p := model.Problem{
		DisplayID:     displayID,
		BankID:        bank.ID,
		Title:         title,
		TimeLimitMS:   timeLimit,
		MemoryLimitMB: memoryLimit,
		Difficulty:    difficulty,
		Points:        points,
		Visibility:    model.ProblemVisibilityPublic,
		CompareMode:   compareMode,
		FloatEps:      floatEps,
		TeamID:        teamID,
		OwnerID:       ownerID,
		Access:        access,
		CreatedBy:     u.ID,
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.InputFormat != nil {
		p.InputFormat = *req.InputFormat
	}
	if req.OutputFormat != nil {
		p.OutputFormat = *req.OutputFormat
	}
	if req.Hint != nil {
		p.Hint = *req.Hint
	}
	if req.Source != nil {
		p.Source = strings.TrimSpace(*req.Source)
	}
	if req.Samples != nil {
		p.SetSamples(*req.Samples)
	} else {
		p.SetSamples(nil)
	}
	if req.Tags != nil {
		p.SetTags(*req.Tags)
	} else {
		p.SetTags(nil)
	}
	if err := h.DB.Create(&p).Error; err != nil {
		httpx.ServerError(c, "创建题目失败")
		return
	}
	httpx.OK(c, gin.H{"problem": gin.H{"id": p.ID, "display_id": p.DisplayID, "kind": p.Kind()}})
}

// UpdateMyProblem 由创建者或队长修改题目内容与访问权限，题号与归属不可改。
func (h *Handler) UpdateMyProblem(c *gin.Context) {
	p, ok := h.loadManageableProblem(c)
	if !ok {
		return
	}
	var req myProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}

	updates := map[string]any{}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			httpx.BadRequest(c, "题目标题不能为空")
			return
		}
		updates["title"] = title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.InputFormat != nil {
		updates["input_format"] = *req.InputFormat
	}
	if req.OutputFormat != nil {
		updates["output_format"] = *req.OutputFormat
	}
	if req.Hint != nil {
		updates["hint"] = *req.Hint
	}
	if req.Samples != nil {
		updates["samples"] = toJSONString(*req.Samples)
	}
	if req.TimeLimitMS != nil {
		if *req.TimeLimitMS < 100 || *req.TimeLimitMS > 30000 {
			httpx.BadRequest(c, "时间限制需在 100 到 30000 毫秒之间")
			return
		}
		updates["time_limit_ms"] = *req.TimeLimitMS
	}
	if req.MemoryLimitMB != nil {
		if *req.MemoryLimitMB < 16 || *req.MemoryLimitMB > 2048 {
			httpx.BadRequest(c, "内存限制需在 16 到 2048 MB 之间")
			return
		}
		updates["memory_limit_mb"] = *req.MemoryLimitMB
	}
	if req.Difficulty != nil {
		updates["difficulty"] = strings.TrimSpace(*req.Difficulty)
	}
	if req.Tags != nil {
		updates["tags"] = toJSONString(*req.Tags)
	}
	if req.Source != nil {
		updates["source"] = strings.TrimSpace(*req.Source)
	}
	if req.Points != nil {
		if *req.Points < 0 || *req.Points > 100000 {
			httpx.BadRequest(c, "题目积分需在 0 到 100000 之间")
			return
		}
		updates["points"] = *req.Points
	}
	if req.CompareMode != nil {
		cm := strings.TrimSpace(*req.CompareMode)
		if cm != model.CompareModeDefault && cm != model.CompareModeExact && cm != model.CompareModeFloat {
			httpx.BadRequest(c, "比对模式取值非法")
			return
		}
		updates["compare_mode"] = cm
	}
	if req.FloatEps != nil {
		updates["float_eps"] = *req.FloatEps
	}
	if req.Access != nil {
		if !validProblemAccessFor(p.TeamID, p.OwnerID, *req.Access) {
			httpx.BadRequest(c, "访问权限取值非法")
			return
		}
		updates["access"] = *req.Access
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&model.Problem{}).Where("id = ?", p.ID).Updates(updates).Error; err != nil {
			httpx.ServerError(c, "更新题目失败")
			return
		}
	}
	var fresh model.Problem
	if err := h.DB.First(&fresh, p.ID).Error; err != nil {
		httpx.ServerError(c, "更新题目失败")
		return
	}
	httpx.OK(c, gin.H{"problem": gin.H{"id": fresh.ID, "display_id": fresh.DisplayID, "kind": fresh.Kind()}})
}

// DeleteMyProblem 软删除自己创建的题目或团队题目（队长），测试数据保留不清空。
func (h *Handler) DeleteMyProblem(c *gin.Context) {
	p, ok := h.loadManageableProblem(c)
	if !ok {
		return
	}
	if err := h.DB.Delete(&model.Problem{}, p.ID).Error; err != nil {
		httpx.ServerError(c, "删除题目失败")
		return
	}
	httpx.OK(c, gin.H{"sync_failed": false, "testdata_retained": true})
}

// ListMyProblemTestdata 返回自己题目的测试点清单。
func (h *Handler) ListMyProblemTestdata(c *gin.Context) {
	p, ok := h.loadManageableProblem(c)
	if !ok {
		return
	}
	h.respondProblemTestdata(c, p)
}

// UploadMyProblemTestdata 上传并整体替换自己题目的测试数据。
func (h *Handler) UploadMyProblemTestdata(c *gin.Context) {
	p, ok := h.loadManageableProblem(c)
	if !ok {
		return
	}
	h.uploadProblemTestdata(c, p)
}

// DeleteMyProblemTestdata 清空自己题目的测试数据。
func (h *Handler) DeleteMyProblemTestdata(c *gin.Context) {
	p, ok := h.loadManageableProblem(c)
	if !ok {
		return
	}
	h.clearProblemTestdata(c, p)
}
