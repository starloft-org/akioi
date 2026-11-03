package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

type adminProblemItem struct {
	ID            uint      `json:"id"`
	DisplayID     string    `json:"display_id"`
	BankID        uint      `json:"bank_id"`
	BankName      string    `json:"bank_name"`
	Title         string    `json:"title"`
	Difficulty    string    `json:"difficulty"`
	Tags          []string  `json:"tags"`
	Source        string    `json:"source"`
	Points        int       `json:"points"`
	Visibility    int       `json:"visibility"`
	TeamID        uint      `json:"team_id"`
	TeamName      string    `json:"team_name"`
	OwnerID       uint      `json:"owner_id"`
	OwnerName     string    `json:"owner_name"`
	Access        int       `json:"access"`
	Kind          string    `json:"kind"`
	AuthorID      uint      `json:"author_id"`
	AuthorName    string    `json:"author_name"`
	TestcaseCount int       `json:"testcase_count"`
	TotalScore    int       `json:"total_score"`
	SubmitCount   int       `json:"submit_count"`
	AcceptedCount int       `json:"accepted_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// AdminListProblems 返回全部题目，包含隐藏题；可用 bank_id 只列某个题库，
// 用 kind 过滤公共题目、团队题目或个人题目。
func (h *Handler) AdminListProblems(c *gin.Context) {
	page, pageSize, offset := httpx.Pagination(c)

	query := h.DB.Model(&model.Problem{})
	// 比赛选题：只能选自己上传的题目，或公开题目（含个人题库、团队题库里的公开题）
	if v := strings.ToLower(strings.TrimSpace(c.Query("contest_selectable"))); v == "1" || v == "true" {
		if u := httpx.CurrentUser(c); u != nil {
			query = query.Where("(created_by = ? OR (visibility = ? AND access = ?))",
				u.ID, model.ProblemVisibilityPublic, model.ProblemAccessPublic)
		}
	}
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
	case model.ProblemKindPublic:
		query = query.Where("team_id = 0 AND owner_id = 0")
	case model.ProblemKindTeam:
		query = query.Where("team_id > 0")
	case model.ProblemKindPersonal:
		query = query.Where("team_id = 0 AND owner_id > 0")
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
	teamNames, ownerNames, err := h.problemOwnerNames(problems)
	if err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}
	authorNames, err := h.problemAuthorNames(problems)
	if err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}

	items := make([]adminProblemItem, 0, len(problems))
	for i := range problems {
		p := &problems[i]
		items = append(items, adminProblemItem{
			ID:            p.ID,
			DisplayID:     p.DisplayID,
			BankID:        p.BankID,
			BankName:      bankNames[p.BankID],
			Title:         p.Title,
			Difficulty:    p.Difficulty,
			Tags:          p.Tags(),
			Source:        p.Source,
			Points:        p.Points,
			Visibility:    p.Visibility,
			TeamID:        p.TeamID,
			TeamName:      teamNames[p.TeamID],
			OwnerID:       p.OwnerID,
			OwnerName:     ownerNames[p.OwnerID],
			Access:        p.Access,
			Kind:          p.Kind(),
			AuthorID:      p.CreatedBy,
			AuthorName:    authorNames[p.CreatedBy],
			TestcaseCount: p.TestcaseCount,
			TotalScore:    totalScore(p),
			SubmitCount:   p.SubmitCount,
			AcceptedCount: p.AcceptedCount,
			CreatedAt:     p.CreatedAt,
		})
	}
	httpx.OK(c, httpx.List(total, page, pageSize, items))
}

// AdminGetProblem 返回题目完整信息。
func (h *Handler) AdminGetProblem(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return
	}
	bankNames, err := h.problemBankNames()
	if err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}
	detail := adminProblemDetail(&p)
	detail["bank_id"] = p.BankID
	detail["bank_name"] = bankNames[p.BankID]
	teamNames, ownerNames, err := h.problemOwnerNames([]model.Problem{p})
	if err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}
	detail["team_name"] = teamNames[p.TeamID]
	detail["owner_name"] = ownerNames[p.OwnerID]
	detail["author_id"] = p.CreatedBy
	detail["author_name"] = h.problemAuthorName(&p)
	httpx.OK(c, gin.H{"problem": detail})
}

func adminProblemDetail(p *model.Problem) gin.H {
	return gin.H{
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
		"visibility":      p.Visibility,
		"compare_mode":    p.CompareMode,
		"spj_language":    p.SPJLanguage,
		"spj_code":        p.SPJCode,
		"float_eps":       p.FloatEps,
		"testcase_count":  p.TestcaseCount,
		"total_score":     totalScore(p),
		"data_hash":       p.DataHash,
		"created_at":      p.CreatedAt,
		"updated_at":      p.UpdatedAt,
		"team_id":         p.TeamID,
		"owner_id":        p.OwnerID,
		"access":          p.Access,
		"kind":            p.Kind(),
	}
}

type adminProblemRequest struct {
	DisplayID *string `json:"display_id"`
	// BankID 归属题库；为空时归属主库
	BankID        *uint           `json:"bank_id"`
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
	Visibility    *int            `json:"visibility"`
	CompareMode   *string         `json:"compare_mode"`
	// SPJCode / SPJLanguage 仅在比对模式为 spj 时需要。
	SPJCode     *string  `json:"spj_code"`
	SPJLanguage *string  `json:"spj_language"`
	FloatEps    *float64 `json:"float_eps"`
	// TeamID 归属团队；0 或空表示公共题目
	TeamID *uint `json:"team_id"`
	// OwnerID 归属个人；0 或空表示非个人题目，与 TeamID 互斥
	OwnerID *uint `json:"owner_id"`
	// Access 访问权限：1 公开，2 仅团队成员或创建者
	Access *int `json:"access"`
}

// AdminCreateProblem 创建题目，题号为空时自动生成。
func (h *Handler) AdminCreateProblem(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req adminProblemRequest
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
	visibility := model.ProblemVisibilityPublic
	if req.Visibility != nil {
		if *req.Visibility != model.ProblemVisibilityPublic && *req.Visibility != model.ProblemVisibilityHidden {
			httpx.BadRequest(c, "可见性取值非法")
			return
		}
		visibility = *req.Visibility
	}
	compareMode := model.CompareModeDefault
	if req.CompareMode != nil {
		cm := strings.TrimSpace(*req.CompareMode)
		if cm != model.CompareModeDefault && cm != model.CompareModeExact && cm != model.CompareModeFloat && cm != model.CompareModeSPJ {
			httpx.BadRequest(c, "比对模式取值非法")
			return
		}
		compareMode = cm
	}
	spjLanguage, spjCode, spjErr := resolveSPJ(compareMode, req.SPJLanguage, req.SPJCode)
	if spjErr != nil {
		httpx.BadRequest(c, spjErr.Error())
		return
	}
	floatEps := 1e-6
	if req.FloatEps != nil {
		floatEps = *req.FloatEps
	}
	difficulty := ""
	if req.Difficulty != nil {
		difficulty = strings.TrimSpace(*req.Difficulty)
	}
	// 积分默认跟随难度，请求里显式给了积分则以请求为准。
	points := model.DefaultPoints(difficulty)
	if req.Points != nil {
		if *req.Points < 0 || *req.Points > 100000 {
			httpx.BadRequest(c, "题目积分需在 0 到 100000 之间")
			return
		}
		points = *req.Points
	}

	// 归属：团队题目与个人题目互斥，都为空时是公共题目
	teamID := uint(0)
	if req.TeamID != nil && *req.TeamID > 0 {
		var teamCount int64
		if err := h.DB.Model(&model.Team{}).Where("id = ?", *req.TeamID).Count(&teamCount).Error; err != nil {
			httpx.ServerError(c, "创建题目失败")
			return
		}
		if teamCount == 0 {
			httpx.BadRequest(c, "归属团队不存在")
			return
		}
		teamID = *req.TeamID
	}
	ownerID := uint(0)
	if req.OwnerID != nil && *req.OwnerID > 0 {
		var userCount int64
		if err := h.DB.Model(&model.User{}).Where("id = ?", *req.OwnerID).Count(&userCount).Error; err != nil {
			httpx.ServerError(c, "创建题目失败")
			return
		}
		if userCount == 0 {
			httpx.BadRequest(c, "归属用户不存在")
			return
		}
		ownerID = *req.OwnerID
	}
	if teamID > 0 && ownerID > 0 {
		httpx.BadRequest(c, "题目不能同时归属团队和用户")
		return
	}
	access := defaultProblemAccess(teamID, ownerID)
	if req.Access != nil {
		if !validProblemAccessFor(teamID, ownerID, *req.Access) {
			httpx.BadRequest(c, "访问权限取值非法")
			return
		}
		access = *req.Access
	}
	// 个人/团队题目不支持积分，只有公共题目的积分参与排行榜。
	if teamID > 0 || ownerID > 0 {
		points = 0
	}

	// 题号由所属题库按「前缀 + 定长序号」自动分配，不接受手工指定；
	// 团队题目用团队库、个人题目用个人库，公共题目用请求指定或主库。
	var bank *model.ProblemBank
	var err error
	if teamID > 0 || ownerID > 0 {
		bank, err = h.problemBankForOwner(teamID, ownerID)
	} else {
		bank, err = h.resolveProblemBank(req.BankID)
	}
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
		Points:        points,
		Visibility:    visibility,
		CompareMode:   compareMode,
		SPJLanguage:   spjLanguage,
		SPJCode:       spjCode,
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
	p.Difficulty = difficulty
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
	// 题目保存后由后端进程在后台推送测试数据，前端不参与。
	h.scheduleProblemSync(p.ID)
	httpx.OK(c, gin.H{"problem": gin.H{"id": p.ID, "display_id": p.DisplayID}})

	if err := h.syncProblemTags(h.DB, p.ID, tagNamesOf(&p)); err != nil {
		httpx.ServerError(c, "保存标签失败")
		return
	}
}

// AdminUpdateProblem 更新题目，仅更新请求中出现的字段。
func (h *Handler) AdminUpdateProblem(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return
	}
	var req adminProblemRequest
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
	// 归属：团队、个人与公共互斥，变更归属后按新归属的题库重新编号
	teamID, ownerID := p.TeamID, p.OwnerID
	ownershipChanged := false
	if req.TeamID != nil && *req.TeamID != teamID {
		teamID = *req.TeamID
		ownershipChanged = true
	}
	if req.OwnerID != nil && *req.OwnerID != ownerID {
		ownerID = *req.OwnerID
		ownershipChanged = true
	}
	if teamID > 0 && ownerID > 0 {
		httpx.BadRequest(c, "题目不能同时归属团队和用户")
		return
	}
	if teamID > 0 {
		var teamCount int64
		if err := h.DB.Model(&model.Team{}).Where("id = ?", teamID).Count(&teamCount).Error; err != nil {
			httpx.ServerError(c, "更新题目失败")
			return
		}
		if teamCount == 0 {
			httpx.BadRequest(c, "归属团队不存在")
			return
		}
	}
	if ownerID > 0 {
		var userCount int64
		if err := h.DB.Model(&model.User{}).Where("id = ?", ownerID).Count(&userCount).Error; err != nil {
			httpx.ServerError(c, "更新题目失败")
			return
		}
		if userCount == 0 {
			httpx.BadRequest(c, "归属用户不存在")
			return
		}
	}

	// 归属题库：请求未指定时沿用原题库，原题库已不存在时回退主库
	var bank *model.ProblemBank
	var err error
	if ownershipChanged && (teamID > 0 || ownerID > 0) {
		bank, err = h.problemBankForOwner(teamID, ownerID)
	} else {
		bank, err = h.resolveProblemBank(&p.BankID)
		if err != nil {
			bank, err = h.resolveProblemBank(nil)
		}
	}
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	// 归属或题库变化后题号需要按新题库规则重新分配
	renumber := false
	if ownershipChanged {
		updates["team_id"] = teamID
		updates["owner_id"] = ownerID
		updates["bank_id"] = bank.ID
		// 个人/团队题目不支持积分，转过去时一并清零。
		if teamID > 0 || ownerID > 0 {
			updates["points"] = 0
		}
		renumber = true
	} else if req.BankID != nil && *req.BankID != 0 && *req.BankID != p.BankID {
		bank, err = h.resolveProblemBank(req.BankID)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		updates["bank_id"] = bank.ID
		renumber = true
	}
	if req.DisplayID != nil {
		displayID := strings.TrimSpace(*req.DisplayID)
		if displayID == "" {
			httpx.BadRequest(c, "题号不能为空")
			return
		}
		if displayID != p.DisplayID {
			if !service.ValidProblemDisplayID(bank, displayID) {
				httpx.BadRequest(c, fmt.Sprintf("题号应为 %s 加上至少 %d 位数字，例如 %s",
					bank.Prefix, bank.CodeLength, service.FormatProblemDisplayID(bank, 0)))
				return
			}
			var exists int64
			// 已删除的题目仍占用题号，查重要连软删除的记录一起算。
			if err := h.DB.Unscoped().Model(&model.Problem{}).
				Where("display_id = ? AND id <> ?", displayID, p.ID).Count(&exists).Error; err != nil {
				httpx.ServerError(c, "更新题目失败")
				return
			}
			if exists > 0 {
				httpx.Conflict(c, "题号已存在")
				return
			}
			updates["display_id"] = displayID
			renumber = false
		}
	}
	if renumber {
		displayID, err := h.allocateProblemDisplayID(bank)
		if err != nil {
			httpx.ServerError(c, "生成题号失败")
			return
		}
		updates["display_id"] = displayID
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
	if req.Points != nil && teamID == 0 && ownerID == 0 {
		if *req.Points < 0 || *req.Points > 100000 {
			httpx.BadRequest(c, "题目积分需在 0 到 100000 之间")
			return
		}
		updates["points"] = *req.Points
	}
	if req.Visibility != nil {
		if *req.Visibility != model.ProblemVisibilityPublic && *req.Visibility != model.ProblemVisibilityHidden {
			httpx.BadRequest(c, "可见性取值非法")
			return
		}
		updates["visibility"] = *req.Visibility
	}
	if req.CompareMode != nil {
		cm := strings.TrimSpace(*req.CompareMode)
		if cm != model.CompareModeDefault && cm != model.CompareModeExact && cm != model.CompareModeFloat && cm != model.CompareModeSPJ {
			httpx.BadRequest(c, "比对模式取值非法")
			return
		}
		updates["compare_mode"] = cm
	}
	if req.SPJCode != nil || req.SPJLanguage != nil {
		mode := p.CompareMode
		if req.CompareMode != nil {
			mode = strings.TrimSpace(*req.CompareMode)
		}
		spjLanguage, spjCode, err := resolveSPJ(mode, req.SPJLanguage, req.SPJCode)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		updates["spj_language"] = spjLanguage
		updates["spj_code"] = spjCode
	}
	if req.FloatEps != nil {
		updates["float_eps"] = *req.FloatEps
	}
	// 访问权限：公共题目只能公开，变更归属时按新归属校正默认权限
	if req.Access != nil {
		if !validProblemAccessFor(teamID, ownerID, *req.Access) {
			httpx.BadRequest(c, "访问权限取值非法")
			return
		}
		updates["access"] = *req.Access
	} else if ownershipChanged && !validProblemAccessFor(teamID, ownerID, p.Access) {
		updates["access"] = defaultProblemAccess(teamID, ownerID)
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
	if req.Tags != nil {
		if err := h.syncProblemTags(h.DB, fresh.ID, tagNamesOf(&fresh)); err != nil {
			httpx.ServerError(c, "保存标签失败")
			return
		}
	}
	// 编辑题目同样在后台推送一次测试数据，保证评测节点上的数据与题目一致。
	h.scheduleProblemSync(fresh.ID)
	httpx.OK(c, gin.H{"problem": gin.H{"id": fresh.ID, "display_id": fresh.DisplayID}})
}

// AdminDeleteProblem 软删除题目：题目记录与测试点清单保留在库中，测试数据文件与评测节点上的数据都不清理。
func (h *Handler) AdminDeleteProblem(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return
	}
	if err := h.DB.Delete(&model.Problem{}, p.ID).Error; err != nil {
		httpx.ServerError(c, "删除题目失败")
		return
	}
	if u := httpx.CurrentUser(c); u != nil {
		logger.Adminf("管理员 %s 删除题目 %s (id=%d)，测试数据保留 ip=%s", u.Username, p.DisplayID, p.ID, c.ClientIP())
	}
	httpx.OK(c, gin.H{"sync_failed": false, "testdata_retained": true})
}

// problemBankNames 返回题库 id 到名称的映射，用于列表展示归属题库。
func (h *Handler) problemBankNames() (map[uint]string, error) {
	var banks []model.ProblemBank
	if err := h.DB.Select("id", "name").Find(&banks).Error; err != nil {
		return nil, err
	}
	names := make(map[uint]string, len(banks))
	for _, b := range banks {
		names[b.ID] = b.Name
	}
	return names, nil
}

// problemOwnerNames 返回题目涉及团队的名称与个人题创建者的用户名。
func (h *Handler) problemOwnerNames(problems []model.Problem) (map[uint]string, map[uint]string, error) {
	teamIDs := make([]uint, 0, len(problems))
	ownerIDs := make([]uint, 0, len(problems))
	seenTeam := map[uint]bool{}
	seenOwner := map[uint]bool{}
	for _, p := range problems {
		if p.TeamID > 0 && !seenTeam[p.TeamID] {
			seenTeam[p.TeamID] = true
			teamIDs = append(teamIDs, p.TeamID)
		}
		if p.OwnerID > 0 && !seenOwner[p.OwnerID] {
			seenOwner[p.OwnerID] = true
			ownerIDs = append(ownerIDs, p.OwnerID)
		}
	}

	teamNames := make(map[uint]string, len(teamIDs))
	if len(teamIDs) > 0 {
		var teams []model.Team
		if err := h.DB.Select("id", "name").Where("id IN ?", teamIDs).Find(&teams).Error; err != nil {
			return nil, nil, err
		}
		for _, t := range teams {
			teamNames[t.ID] = t.Name
		}
	}

	ownerNames := make(map[uint]string, len(ownerIDs))
	if len(ownerIDs) > 0 {
		var users []model.User
		if err := h.DB.Select("id", "username").Where("id IN ?", ownerIDs).Find(&users).Error; err != nil {
			return nil, nil, err
		}
		for _, u := range users {
			ownerNames[u.ID] = u.Username
		}
	}
	return teamNames, ownerNames, nil
}

// resolveProblemBank 取指定题库；id 为空或 0 时取主库。
func (h *Handler) resolveProblemBank(id *uint) (*model.ProblemBank, error) {
	var bank model.ProblemBank
	if id == nil || *id == 0 {
		// 主库是迁移预置的公共题目题库（角色 primary），公共题目默认落到这里
		err := h.DB.
			Where("preset_role = ?", model.ProblemBankRolePrimary).
			Order("id asc").First(&bank).Error
		if err != nil {
			if err := h.DB.Order("id asc").First(&bank).Error; err != nil {
				return nil, fmt.Errorf("尚未创建题库，请先在题库管理中新建题库")
			}
		}
		return &bank, nil
	}
	if err := h.DB.First(&bank, *id).Error; err != nil {
		return nil, fmt.Errorf("题库不存在")
	}
	return &bank, nil
}

// allocateProblemDisplayID 生成题库内的下一个题号；并发写入撞号时重试，仍冲突则报错。
func (h *Handler) allocateProblemDisplayID(bank *model.ProblemBank) (string, error) {
	return h.allocateProblemDisplayIDTx(h.DB, bank)
}

type testcaseItem struct {
	Name       string `json:"name"`
	GroupID    int    `json:"group_id"`
	Index      int    `json:"index"`
	Score      int    `json:"score"`
	InputSize  int64  `json:"input_size"`
	OutputSize int64  `json:"output_size"`
	InputHash  string `json:"input_hash"`
	OutputHash string `json:"output_hash"`
}

// AdminImportProblem 从 zip 题目包导入题目：题面取 problem.md，元信息取 config.json，数据取 *.in/*.out。
// importItem 是单个题目包的导入结果。
type importItem struct {
	Filename      string   `json:"filename"`
	OK            bool     `json:"ok"`
	ProblemID     uint     `json:"problem_id,omitempty"`
	DisplayID     string   `json:"display_id,omitempty"`
	Title         string   `json:"title,omitempty"`
	TestcaseCount int      `json:"testcase_count,omitempty"`
	SyncFailed    []string `json:"sync_failed,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// AdminImportProblem 导入题目包，支持一次上传多个 zip；逐包导入，单包失败不影响其余包。
// 表单字段 files 为多包（重复字段名），也兼容旧客户端的单文件字段 file。
func (h *Handler) AdminImportProblem(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		httpx.BadRequest(c, "请上传题目包 zip")
		return
	}
	fileHeaders := form.File["files"]
	if len(fileHeaders) == 0 {
		fileHeaders = form.File["file"]
	}
	if len(fileHeaders) == 0 {
		httpx.BadRequest(c, "请上传题目包 zip")
		return
	}
	// 导入的题目统一归入请求指定的题库，未指定时归入主库
	var bankID *uint
	if vals := form.Value["bank_id"]; len(vals) > 0 {
		if raw := strings.TrimSpace(vals[0]); raw != "" {
			n, err := strconv.ParseUint(raw, 10, 64)
			if err != nil {
				httpx.BadRequest(c, "题库参数非法")
				return
			}
			id := uint(n)
			bankID = &id
		}
	}
	bank, err := h.resolveProblemBank(bankID)
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}

	// 可选：整批导入为隐藏题。隐藏题不出现在公共题单，普通用户看不到也提交不了，
	// 只有在已开始的比赛里引用该题时才可访问。
	visibility := model.ProblemVisibilityPublic
	if vals := form.Value["visibility"]; len(vals) > 0 {
		switch strings.ToLower(strings.TrimSpace(vals[0])) {
		case "hidden", "2":
			visibility = model.ProblemVisibilityHidden
		case "", "public", "1":
		default:
			httpx.BadRequest(c, "可见性参数非法")
			return
		}
	}

	// 导入在后台顺序执行：上传完立即返回任务号，前端轮询进度，避免大批量导入把请求拖到网关超时。
	maxBytes := int64(h.Settings.Current().MaxTestdataMB) * 1024 * 1024
	staged, cleanup, err := saveStagedFiles(fileHeaders, maxBytes)
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	// 支持把多个题目包打成一个外层 zip 上传：这里先展开成一个个题目包再排队导入。
	expanded, packed, packedCleanup, err := expandImportContainers(staged, maxBytes)
	if err != nil {
		cleanup()
		httpx.BadRequest(c, err.Error())
		return
	}
	if packed {
		savedCleanup := cleanup
		cleanup = func() {
			packedCleanup()
			savedCleanup()
		}
		staged = expanded
	}
	ip := c.ClientIP()
	task := createImportTask(len(staged), 0)

	// 导入一律交给后台任务：上传完立刻返回任务号，前端轮询进度。
	// 即使单个包很大（测试点多、需要推送评测节点），也不会有请求被网关或浏览器超时掐断。

	go func() {
		defer cleanup()
		h.runProblemImport(task.ID, staged, u, bank, maxBytes, ip, importOwnerPublic(visibility), packed)
	}()
	if packed {
		logger.Adminf("管理员 %s 从打包文件开始导入 %d 个题目包 ip=%s", u.Username, task.Total, ip)
	} else {
		logger.Adminf("管理员 %s 开始批量导入 %d 个题目包 ip=%s", u.Username, task.Total, ip)
	}
	httpx.OK(c, gin.H{
		"task_id":   task.ID,
		"total":     task.Total,
		"status":    task.Status,
		"container": packed,
	})
}

// AdminSyncTestdata 主动把某题的测试数据推送到全部评测节点，用于节点恢复后的补推送。
func (h *Handler) AdminSyncTestdata(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return
	}
	if p.TestcaseCount == 0 {
		httpx.BadRequest(c, "该题还没有测试数据")
		return
	}
	if len(h.Settings.Current().JudgeNodeEndpoints) == 0 {
		httpx.BadRequest(c, "未配置评测节点，请先在系统设置中添加")
		return
	}
	failed := h.Testdata.SyncToNodes(syncContext(c), p.ID)
	if u := httpx.CurrentUser(c); u != nil {
		logger.Adminf("管理员 %s 手动推送题目 %d 测试数据，%d 个节点失败", u.Username, p.ID, len(failed))
	}
	if len(failed) > 0 {
		httpx.OK(c, gin.H{"synced": false, "sync_failed": failed})
		return
	}
	httpx.OK(c, gin.H{"synced": true, "sync_failed": []string{}})
}

// AdminListTestdata 返回某题的测试点清单。
func (h *Handler) AdminListTestdata(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return
	}
	h.respondProblemTestdata(c, &p)
}

func (h *Handler) testcaseItems(problemID uint) ([]testcaseItem, error) {
	cases, err := h.Testdata.List(problemID)
	if err != nil {
		return nil, err
	}
	items := make([]testcaseItem, 0, len(cases))
	for _, tc := range cases {
		name := tc.Name
		if name == "" {
			name = strconv.Itoa(tc.Index)
		}
		items = append(items, testcaseItem{
			Name:       name,
			GroupID:    tc.GroupID,
			Index:      tc.Index,
			Score:      tc.Score,
			InputSize:  tc.InputSize,
			OutputSize: tc.OutputSize,
			InputHash:  tc.InputHash,
			OutputHash: tc.OutputHash,
		})
	}
	return items, nil
}

func testcaseTotalScore(items []testcaseItem) int {
	total := 0
	for i := range items {
		total += items[i].Score
	}
	return total
}

// AdminUploadTestdata 上传并整体替换某题的测试数据，支持单 zip 与多文件两种形式。
func (h *Handler) AdminUploadTestdata(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return
	}
	h.uploadProblemTestdata(c, &p)
}

// AdminDeleteTestdata 清空某题的测试数据。
func (h *Handler) AdminDeleteTestdata(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return
	}
	h.clearProblemTestdata(c, &p)
}

// syncContext 以不随客户端断连取消的上下文同步数据，保证推送不会半途中断。
func syncContext(c *gin.Context) context.Context {
	return context.WithoutCancel(c.Request.Context())
}

// AdminDownloadTestdata 打包下载某题的测试数据。
func (h *Handler) AdminDownloadTestdata(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, id).Error; err != nil {
		httpx.NotFound(c, "题目不存在")
		return
	}
	buf, err := h.Testdata.Zip(p.ID)
	if err != nil {
		httpx.NotFound(c, "测试数据不存在")
		return
	}
	c.Header("Content-Disposition", "attachment; filename=problem-"+p.DisplayID+"-testdata.zip")
	c.Data(http.StatusOK, "application/zip", buf)
}

// readUploadedFile 读取上传文件并校验单文件大小上限。
func readUploadedFile(fh *multipart.FileHeader, maxBytes int64) ([]byte, error) {
	if maxBytes > 0 && fh.Size > maxBytes {
		return nil, fmt.Errorf("文件 %s 超过大小上限", fh.Filename)
	}
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// toJSONString 以 JSON 序列化值，失败时返回空数组字面量。
func toJSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}
