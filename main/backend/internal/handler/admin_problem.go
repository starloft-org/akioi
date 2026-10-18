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
	points := 100
	if req.Points != nil {
		if *req.Points < 0 || *req.Points > 100000 {
			httpx.BadRequest(c, "题目积分需在 0 到 100000 之间")
			return
		}
		points = *req.Points
	}

	displayID := ""
	if req.DisplayID != nil {
		displayID = strings.TrimSpace(*req.DisplayID)
	}
	if displayID == "" {
		next, err := h.nextProblemDisplayID()
		if err != nil {
			httpx.ServerError(c, "生成题号失败")
			return
		}
		displayID = next
	} else {
		var exists int64
		if err := h.DB.Model(&model.Problem{}).Where("display_id = ?", displayID).Count(&exists).Error; err != nil {
			httpx.ServerError(c, "创建题目失败")
			return
		}
		if exists > 0 {
			httpx.Conflict(c, "题号已存在")
			return
		}
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
	if req.Difficulty != nil {
		p.Difficulty = strings.TrimSpace(*req.Difficulty)
	}
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

// nextProblemDisplayID 从 P1001 起递增，返回首个未被占用的题号。
func (h *Handler) nextProblemDisplayID() (string, error) {
	for n := 1001; ; n++ {
		displayID := "P" + strconv.Itoa(n)
		var count int64
		if err := h.DB.Model(&model.Problem{}).Where("display_id = ?", displayID).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return displayID, nil
		}
	}
}

type testcaseItem struct {
	Index      int    `json:"index"`
	Score      int    `json:"score"`
	InputSize  int64  `json:"input_size"`
	OutputSize int64  `json:"output_size"`
	InputHash  string `json:"input_hash"`
	OutputHash string `json:"output_hash"`
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
