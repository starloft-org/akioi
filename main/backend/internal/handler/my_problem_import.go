package handler

import (
	"strings"

	"akioi/internal/httpx"
	"akioi/internal/logger"

	"github.com/gin-gonic/gin"
)

// MyImportProblem 用户端批量导入个人题目。
//
// 包格式与后台导入完全一致，区别在于归属：题目进入个人题库（P 前缀），
// OwnerID 是本用户，默认访问权限为「仅自己」。上传完立即返回任务号，前端轮询进度。
func (h *Handler) MyImportProblem(c *gin.Context) {
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
	// 用户端导入只能落在个人题库，题号由个人库统一分配
	bank, err := h.problemBankForOwner(0, u.ID)
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}

	maxBytes := int64(h.Settings.Current().MaxTestdataMB) * 1024 * 1024
	staged, cleanup, err := saveStagedFiles(fileHeaders, maxBytes)
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	// 支持把一个外层 zip 直接上传，里面放多个题目包
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
	task := createImportTask(len(staged), u.ID)

	go func() {
		defer cleanup()
		h.runProblemImport(task.ID, staged, u, bank, maxBytes, ip, importOwnerPersonal(u), packed)
	}()
	logger.Adminf("用户 %s 开始导入 %d 个个人题目包 ip=%s", u.Username, task.Total, ip)
	httpx.OK(c, gin.H{
		"task_id":   task.ID,
		"total":     task.Total,
		"status":    task.Status,
		"container": packed,
	})
}

// MyProblemImportStatus 返回本人导入任务的进度；别人的任务一律当作不存在。
func (h *Handler) MyProblemImportStatus(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	task, ok := snapshotImportTask(strings.TrimSpace(c.Param("id")))
	if !ok || (task.OwnerID != u.ID && !u.IsAdmin()) {
		httpx.NotFound(c, "导入任务不存在或已过期")
		return
	}
	httpx.OK(c, task)
}
