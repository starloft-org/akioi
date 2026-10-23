package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
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
	Title         string    `json:"title"`
	Difficulty    string    `json:"difficulty"`
	Tags          []string  `json:"tags"`
	Source        string    `json:"source"`
	Points        int       `json:"points"`
	Visibility    int       `json:"visibility"`
	ContestOnly   bool      `json:"contest_only"`
	TestcaseCount int       `json:"testcase_count"`
	TotalScore    int       `json:"total_score"`
	SubmitCount   int       `json:"submit_count"`
	AcceptedCount int       `json:"accepted_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// AdminListProblems 返回全部题目，包含隐藏题。
func (h *Handler) AdminListProblems(c *gin.Context) {
	page, pageSize, offset := httpx.Pagination(c)

	query := h.DB.Model(&model.Problem{})
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("display_id = ? OR title LIKE ?", kw, like)
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

	items := make([]adminProblemItem, 0, len(problems))
	for i := range problems {
		p := &problems[i]
		items = append(items, adminProblemItem{
			ID:            p.ID,
			DisplayID:     p.DisplayID,
			Title:         p.Title,
			Difficulty:    p.Difficulty,
			Tags:          p.Tags(),
			Source:        p.Source,
			Points:        p.Points,
			Visibility:    p.Visibility,
			ContestOnly:   p.ContestOnly,
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
	httpx.OK(c, gin.H{"problem": adminProblemDetail(&p)})
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
		"contest_only":    p.ContestOnly,
		"compare_mode":    p.CompareMode,
		"float_eps":       p.FloatEps,
		"testcase_count":  p.TestcaseCount,
		"total_score":     totalScore(p),
		"data_hash":       p.DataHash,
		"created_at":      p.CreatedAt,
		"updated_at":      p.UpdatedAt,
		"team_id":         p.TeamID,
	}
}

type adminProblemRequest struct {
	DisplayID     *string         `json:"display_id"`
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
	ContestOnly   *bool           `json:"contest_only"`
	CompareMode   *string         `json:"compare_mode"`
	FloatEps      *float64        `json:"float_eps"`
	// TeamID 归属团队；0 或空表示公共题目
	TeamID        *uint           `json:"team_id"`
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
	// 积分默认跟随难度，请求里显式给了积分则以请求为准。
	points := model.DefaultPoints(difficulty)
	if req.Points != nil {
		if *req.Points < 0 || *req.Points > 100000 {
			httpx.BadRequest(c, "题目积分需在 0 到 100000 之间")
			return
		}
		points = *req.Points
	}

	// 题号由系统按 A0000 起自动递增分配，不再接受手工指定
	displayID, err := h.nextProblemDisplayID()
	if err != nil {
		httpx.ServerError(c, "生成题号失败")
		return
	}

	p := model.Problem{
		DisplayID:     displayID,
		Title:         title,
		TimeLimitMS:   timeLimit,
		MemoryLimitMB: memoryLimit,
		Points:        points,
		Visibility:    visibility,
		CompareMode:   compareMode,
		FloatEps:      floatEps,
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
	if req.ContestOnly != nil {
		p.ContestOnly = *req.ContestOnly
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
		p.TeamID = *req.TeamID
	}

	if err := h.DB.Create(&p).Error; err != nil {
		httpx.ServerError(c, "创建题目失败")
		return
	}
	httpx.OK(c, gin.H{"problem": gin.H{"id": p.ID, "display_id": p.DisplayID}})
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
	if req.DisplayID != nil {
		displayID := strings.TrimSpace(*req.DisplayID)
		if displayID == "" {
			httpx.BadRequest(c, "题号不能为空")
			return
		}
		if !validProblemDisplayID(displayID) {
			httpx.BadRequest(c, "题号格式应为 A 加至少 4 位数字，例如 A0000")
			return
		}
		var exists int64
		if err := h.DB.Model(&model.Problem{}).
			Where("display_id = ? AND id <> ?", displayID, p.ID).Count(&exists).Error; err != nil {
			httpx.ServerError(c, "更新题目失败")
			return
		}
		if exists > 0 {
			httpx.Conflict(c, "题号已存在")
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
	if req.Points != nil {
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
	if req.ContestOnly != nil {
		updates["contest_only"] = *req.ContestOnly
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
	// 归属团队：传 0 表示移出团队，变为公共题目
	if req.TeamID != nil {
		if *req.TeamID > 0 {
			var teamCount int64
			if err := h.DB.Model(&model.Team{}).Where("id = ?", *req.TeamID).Count(&teamCount).Error; err != nil {
				httpx.ServerError(c, "更新题目失败")
				return
			}
			if teamCount == 0 {
				httpx.BadRequest(c, "归属团队不存在")
				return
			}
		}
		updates["team_id"] = *req.TeamID
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
	httpx.OK(c, gin.H{"problem": gin.H{"id": fresh.ID, "display_id": fresh.DisplayID}})
}

// AdminDeleteProblem 删除题目及其测试数据。
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
	if err := h.Testdata.Delete(p.ID); err != nil {
		httpx.ServerError(c, "删除测试数据失败")
		return
	}
	if err := h.DB.Delete(&model.Problem{}, p.ID).Error; err != nil {
		httpx.ServerError(c, "删除题目失败")
		return
	}
	httpx.OK(c, gin.H{"sync_failed": h.Testdata.ClearOnNodes(syncContext(c), p.ID)})
}

// nextProblemDisplayID 返回 A 开头的下一个题号：取现有最大编号加一，空库从 A0000 开始。
func (h *Handler) nextProblemDisplayID() (string, error) {
	var ids []string
	if err := h.DB.Model(&model.Problem{}).
		Where("display_id LIKE 'A%'").
		Pluck("display_id", &ids).Error; err != nil {
		return "", err
	}
	next := 0
	for _, id := range ids {
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "A")); err == nil && n >= next {
			next = n + 1
		}
	}
	return fmt.Sprintf("A%04d", next), nil
}

// validProblemDisplayID 校验题号形如 A0000：字母 A 加至少 4 位数字。
func validProblemDisplayID(id string) bool {
	if len(id) < 5 || id[0] != 'A' {
		return false
	}
	for _, c := range id[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

type testcaseItem struct {
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

	maxBytes := int64(h.Settings.Current().MaxTestdataMB) * 1024 * 1024
	items := make([]importItem, 0, len(fileHeaders))
	successCount := 0
	for _, fh := range fileHeaders {
		item := h.importOneProblem(c, fh, u, maxBytes)
		if item.OK {
			successCount++
		}
		items = append(items, item)
	}

	resp := gin.H{
		"items":         items,
		"total":         len(fileHeaders),
		"success_count": successCount,
		"failed_count":  len(fileHeaders) - successCount,
	}
	// 单包时保留旧响应结构，兼容已有前端流程
	if len(fileHeaders) == 1 && items[0].OK {
		resp["problem"] = gin.H{"id": items[0].ProblemID, "display_id": items[0].DisplayID}
		resp["title"] = items[0].Title
		resp["testcase_count"] = items[0].TestcaseCount
		resp["sync_failed"] = items[0].SyncFailed
	}
	httpx.OK(c, resp)
}

// importOneProblem 导入单个题目包；测试数据失败时回滚已创建的题目。
func (h *Handler) importOneProblem(c *gin.Context, fh *multipart.FileHeader, u *model.User, maxBytes int64) importItem {
	item := importItem{Filename: fh.Filename}
	if !strings.EqualFold(filepath.Ext(fh.Filename), ".zip") {
		item.Error = "题目包必须是 zip 文件"
		return item
	}
	content, err := readUploadedFile(fh, maxBytes)
	if err != nil {
		item.Error = err.Error()
		return item
	}
	files, err := service.ExtractZip(content)
	if err != nil {
		item.Error = err.Error()
		return item
	}
	pkg, err := service.ParseProblemPackage(files)
	if err != nil {
		item.Error = err.Error()
		return item
	}

	// 题号统一自动分配，忽略导入包中的 display_id
	displayID, err := h.nextProblemDisplayID()
	if err != nil {
		item.Error = "生成题号失败"
		return item
	}
	var exists int64
	if err := h.DB.Model(&model.Problem{}).Where("display_id = ?", displayID).Count(&exists).Error; err != nil {
		item.Error = "导入题目失败"
		return item
	}
	if exists > 0 {
		item.Error = "题号 " + displayID + " 已存在"
		return item
	}

	timeLimit := pkg.Config.TimeLimitMS
	if timeLimit < 100 || timeLimit > 30000 {
		timeLimit = 1000
	}
	memoryLimit := pkg.Config.MemoryLimitMB
	if memoryLimit < 16 || memoryLimit > 2048 {
		memoryLimit = 256
	}
	visibility := pkg.Config.Visibility
	if visibility != model.ProblemVisibilityHidden {
		visibility = model.ProblemVisibilityPublic
	}
	difficulty := strings.TrimSpace(pkg.Config.Difficulty)
	points := pkg.Config.Points
	if points <= 0 {
		points = model.DefaultPoints(difficulty)
	}

	p := model.Problem{
		DisplayID:     displayID,
		Title:         strings.TrimSpace(pkg.Config.Title),
		Description:   pkg.Description,
		InputFormat:   pkg.InputFormat,
		OutputFormat:  pkg.OutputFormat,
		Hint:          pkg.Hint,
		TimeLimitMS:   timeLimit,
		MemoryLimitMB: memoryLimit,
		Difficulty:    difficulty,
		Source:        strings.TrimSpace(pkg.Config.Source),
		Points:        points,
		Visibility:    visibility,
		CompareMode:   model.CompareModeDefault,
		FloatEps:      1e-6,
		CreatedBy:     u.ID,
	}
	p.SetSamples(pkg.Samples)
	p.SetTags(pkg.Config.Tags)

	if err := h.DB.Create(&p).Error; err != nil {
		item.Error = "导入题目失败"
		return item
	}

	// 包里带测试数据才写入；只有题面时允许先建题，之后再单独上传数据。
	hasTestdata := false
	for name := range files {
		switch strings.ToLower(filepath.Ext(name)) {
		case ".in", ".out", ".ans":
			hasTestdata = true
		}
	}
	if hasTestdata {
		if err := h.Testdata.Replace(&p, files, nil); err != nil {
			h.DB.Delete(&model.Problem{}, p.ID)
			item.Error = "导入测试数据失败: " + err.Error()
			return item
		}
	}

	logger.Adminf("管理员 %s 导入题目 %s (id=%d) ip=%s", u.Username, p.DisplayID, p.ID, c.ClientIP())
	item.OK = true
	item.ProblemID = p.ID
	item.DisplayID = p.DisplayID
	item.Title = p.Title
	item.TestcaseCount = p.TestcaseCount
	item.SyncFailed = h.Testdata.SyncToNodes(syncContext(c), p.ID)
	return item
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
	items, err := h.testcaseItems(p.ID)
	if err != nil {
		httpx.ServerError(c, "查询测试数据失败")
		return
	}
	httpx.OK(c, gin.H{"total_score": testcaseTotalScore(items), "items": items})
}

func (h *Handler) testcaseItems(problemID uint) ([]testcaseItem, error) {
	cases, err := h.Testdata.List(problemID)
	if err != nil {
		return nil, err
	}
	items := make([]testcaseItem, 0, len(cases))
	for _, tc := range cases {
		items = append(items, testcaseItem{
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

	form, err := c.MultipartForm()
	if err != nil {
		httpx.BadRequest(c, "解析上传内容失败")
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		httpx.BadRequest(c, "缺少测试数据文件")
		return
	}
	maxBytes := int64(h.Settings.Current().MaxTestdataMB) * 1024 * 1024

	var data map[string][]byte
	if len(files) == 1 && strings.EqualFold(filepath.Ext(files[0].Filename), ".zip") {
		content, err := readUploadedFile(files[0], maxBytes)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		data, err = service.ExtractZip(content)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
	} else {
		data = make(map[string][]byte, len(files))
		for _, fh := range files {
			content, err := readUploadedFile(fh, maxBytes)
			if err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
			data[fh.Filename] = content
		}
	}

	var scores []int
	if raw := form.Value["scores"]; len(raw) > 0 && strings.TrimSpace(raw[0]) != "" {
		if err := json.Unmarshal([]byte(raw[0]), &scores); err != nil {
			httpx.BadRequest(c, "分值格式错误")
			return
		}
	}

	if err := h.Testdata.Replace(&p, data, scores); err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	items, err := h.testcaseItems(p.ID)
	if err != nil {
		httpx.ServerError(c, "查询测试数据失败")
		return
	}
	httpx.OK(c, gin.H{
		"items":       items,
		"total_score": testcaseTotalScore(items),
		"sync_failed": h.Testdata.SyncToNodes(syncContext(c), p.ID),
	})
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
	if err := h.Testdata.Delete(p.ID); err != nil {
		httpx.ServerError(c, "删除测试数据失败")
		return
	}
	httpx.OK(c, gin.H{"sync_failed": h.Testdata.ClearOnNodes(syncContext(c), p.ID)})
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
