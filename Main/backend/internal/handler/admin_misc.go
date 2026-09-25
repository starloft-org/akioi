package handler

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/model"
	"akioi/internal/queue"

	"github.com/gin-gonic/gin"
)

// AdminStats 返回后台首页的汇总统计。
func (h *Handler) AdminStats(c *gin.Context) {
	var userCount, problemCount, submissionCount, acceptedCount, todayCount, testcaseCount int64
	h.DB.Model(&model.User{}).Count(&userCount)
	h.DB.Model(&model.Problem{}).Count(&problemCount)
	h.DB.Model(&model.Submission{}).Count(&submissionCount)
	h.DB.Model(&model.Submission{}).Where("status = ?", model.StatusAccepted).Count(&acceptedCount)
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	h.DB.Model(&model.Submission{}).Where("created_at >= ?", startOfDay).Count(&todayCount)
	h.DB.Model(&model.Testcase{}).Count(&testcaseCount)

	queueLength, err := queue.New(h.RDB, h.Cfg.Judge.QueueKey).Length(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, "查询评测队列失败")
		return
	}

	acceptanceRate := 0.0
	if submissionCount > 0 {
		acceptanceRate = float64(acceptedCount) * 100 / float64(submissionCount)
	}

	httpx.OK(c, gin.H{
		"user_count":             userCount,
		"problem_count":          problemCount,
		"submission_count":       submissionCount,
		"accepted_count":         acceptedCount,
		"acceptance_rate":        acceptanceRate,
		"today_submission_count": todayCount,
		"queue_length":           queueLength,
		"testcase_count":         testcaseCount,
	})
}

// adminLogTypes 是允许读取的日志分类白名单，避免路径穿越。
var adminLogTypes = map[string]bool{
	"business": true,
	"access":   true,
	"admin":    true,
	"judge":    true,
	"error":    true,
}

// AdminLogs 返回指定分类日志文件的最后若干行。
func (h *Handler) AdminLogs(c *gin.Context) {
	logType := strings.TrimSpace(c.Query("type"))
	if logType == "" {
		logType = "business"
	}
	if !adminLogTypes[logType] {
		httpx.BadRequest(c, "不支持的日志类型")
		return
	}
	lines := httpx.QueryInt(c, "lines", 200)
	if lines <= 0 {
		lines = 200
	}
	if lines > 2000 {
		lines = 2000
	}

	out, err := readLastLines(filepath.Join(h.Cfg.LogDir, logType+".log"), lines)
	if err != nil {
		if os.IsNotExist(err) {
			httpx.OK(c, gin.H{"type": logType, "lines": []string{}})
			return
		}
		httpx.ServerError(c, "读取日志失败")
		return
	}
	httpx.OK(c, gin.H{"type": logType, "lines": out})
}

// readLastLines 从文件尾部读取最多 maxLines 行，单次最多回溯 4MB。
func readLastLines(path string, maxLines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	const maxRead = int64(4 << 20)
	start := int64(0)
	if info.Size() > maxRead {
		start = info.Size() - maxRead
	}
	data, err := io.ReadAll(io.NewSectionReader(f, start, info.Size()-start))
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	if start > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	if lines == nil {
		lines = []string{}
	}
	return lines, nil
}

type adminAnnouncementItem struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Visible   bool      `json:"visible"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AdminListAnnouncements 返回全部公告，包含不可见项。
func (h *Handler) AdminListAnnouncements(c *gin.Context) {
	page, pageSize, offset := httpx.Pagination(c)

	var total int64
	if err := h.DB.Model(&model.Announcement{}).Count(&total).Error; err != nil {
		httpx.ServerError(c, "查询公告失败")
		return
	}
	var rows []model.Announcement
	if err := h.DB.Order("id desc").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		httpx.ServerError(c, "查询公告失败")
		return
	}
	items := make([]adminAnnouncementItem, 0, len(rows))
	for i := range rows {
		items = append(items, adminAnnouncementItem{
			ID:        rows[i].ID,
			Title:     rows[i].Title,
			Content:   rows[i].Content,
			Visible:   rows[i].Visible,
			CreatedAt: rows[i].CreatedAt,
			UpdatedAt: rows[i].UpdatedAt,
		})
	}
	httpx.OK(c, httpx.List(total, page, pageSize, items))
}

type adminAnnouncementRequest struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
	Visible *bool   `json:"visible"`
}

// AdminCreateAnnouncement 创建公告。
func (h *Handler) AdminCreateAnnouncement(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req adminAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	title := ""
	if req.Title != nil {
		title = strings.TrimSpace(*req.Title)
	}
	if title == "" {
		httpx.BadRequest(c, "公告标题不能为空")
		return
	}

	a := model.Announcement{Title: title, CreatedBy: u.ID, Visible: true}
	if req.Content != nil {
		a.Content = *req.Content
	}
	if req.Visible != nil {
		a.Visible = *req.Visible
	}
	if err := h.DB.Create(&a).Error; err != nil {
		httpx.ServerError(c, "创建公告失败")
		return
	}
	httpx.OK(c, gin.H{"announcement": gin.H{"id": a.ID}})
}

// AdminUpdateAnnouncement 更新公告，仅更新请求中出现的字段。
func (h *Handler) AdminUpdateAnnouncement(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var a model.Announcement
	if err := h.DB.First(&a, id).Error; err != nil {
		httpx.NotFound(c, "公告不存在")
		return
	}
	var req adminAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}

	updates := map[string]any{}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			httpx.BadRequest(c, "公告标题不能为空")
			return
		}
		updates["title"] = title
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.Visible != nil {
		updates["visible"] = *req.Visible
	}
	if len(updates) > 0 {
		if err := h.DB.Model(&model.Announcement{}).Where("id = ?", a.ID).Updates(updates).Error; err != nil {
			httpx.ServerError(c, "更新公告失败")
			return
		}
	}
	httpx.OK(c, gin.H{"announcement": gin.H{"id": a.ID}})
}

// AdminDeleteAnnouncement 删除公告。
func (h *Handler) AdminDeleteAnnouncement(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var a model.Announcement
	if err := h.DB.First(&a, id).Error; err != nil {
		httpx.NotFound(c, "公告不存在")
		return
	}
	if err := h.DB.Delete(&model.Announcement{}, a.ID).Error; err != nil {
		httpx.ServerError(c, "删除公告失败")
		return
	}
	httpx.OK(c, gin.H{})
}
