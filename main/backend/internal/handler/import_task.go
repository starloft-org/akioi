package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"sync"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
)

// 批量导入题目在后台执行：上传完成后立即返回任务号，前端轮询进度。
// 这样一千个题目包也不会卡住一个 HTTP 请求，避免网关超时导致 Network Error。
const (
	importTaskRunning     = "running"
	importTaskDone        = "done"
	importTaskKeepPeriod  = time.Hour
	importTaskMaxRetained = 50
)

// ImportTask 是一次批量导入的进度快照。
type ImportTask struct {
	ID           string       `json:"task_id"`
	Total        int          `json:"total"`
	Done         int          `json:"done"`
	SuccessCount int          `json:"success_count"`
	FailedCount  int          `json:"failed_count"`
	Status       string       `json:"status"`
	Message      string       `json:"message,omitempty"`
	Items        []importItem `json:"items"`
	CreatedAt    time.Time    `json:"created_at"`
	FinishedAt   *time.Time   `json:"finished_at,omitempty"`
}

var (
	importTasksMu sync.Mutex
	importTasks   = make(map[string]*ImportTask)
)

// createImportTask 登记一个新任务并返回快照。
func createImportTask(total int) ImportTask {
	task := &ImportTask{
		ID:        newImportTaskID(),
		Total:     total,
		Status:    importTaskRunning,
		Items:     make([]importItem, 0, total),
		CreatedAt: time.Now(),
	}
	importTasksMu.Lock()
	defer importTasksMu.Unlock()
	gcImportTasksLocked(time.Now())
	importTasks[task.ID] = task
	snapshot := *task
	return snapshot
}

// updateImportTask 在锁内更新任务，调用方只改自己关心的字段。
func updateImportTask(id string, apply func(task *ImportTask)) {
	importTasksMu.Lock()
	defer importTasksMu.Unlock()
	task, ok := importTasks[id]
	if !ok {
		return
	}
	apply(task)
}

// snapshotImportTask 返回进度副本，避免前端读到正在写入的切片。
func snapshotImportTask(id string) (ImportTask, bool) {
	importTasksMu.Lock()
	defer importTasksMu.Unlock()
	task, ok := importTasks[id]
	if !ok {
		return ImportTask{}, false
	}
	snapshot := *task
	snapshot.Items = append([]importItem(nil), task.Items...)
	return snapshot, true
}

// importTaskResponse 把任务快照转成接口字段，同步导入与轮询共用同一份结构。
func importTaskResponse(task ImportTask) gin.H {
	return gin.H{
		"task_id":       task.ID,
		"total":         task.Total,
		"done":          task.Done,
		"success_count": task.SuccessCount,
		"failed_count":  task.FailedCount,
		"status":        task.Status,
		"items":         task.Items,
	}
}

// gcImportTasksLocked 清掉过期任务，同时限制内存里保留的任务数量。
func gcImportTasksLocked(now time.Time) {
	expired := make([]string, 0, len(importTasks))
	for id, task := range importTasks {
		if task.Status == importTaskDone && task.FinishedAt != nil && now.Sub(*task.FinishedAt) > importTaskKeepPeriod {
			expired = append(expired, id)
		}
	}
	for _, id := range expired {
		delete(importTasks, id)
	}
	if len(importTasks) <= importTaskMaxRetained {
		return
	}
	oldest := ""
	var oldestAt time.Time
	for id, task := range importTasks {
		if task.Status != importTaskDone {
			continue
		}
		if oldest == "" || task.CreatedAt.Before(oldestAt) {
			oldest, oldestAt = id, task.CreatedAt
		}
	}
	if oldest != "" {
		delete(importTasks, oldest)
	}
}

func newImportTaskID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// stagedImportFile 是落到临时目录的上传包，后台任务从磁盘读取，不占用请求生命周期。
type stagedImportFile struct {
	Name string
	Path string
}

// AdminProblemImportStatus 返回批量导入的进度与结果。
func (h *Handler) AdminProblemImportStatus(c *gin.Context) {
	id := c.Param("id")
	task, ok := snapshotImportTask(id)
	if !ok {
		httpx.NotFound(c, "导入任务不存在或已过期")
		return
	}
	httpx.OK(c, task)
}

// runProblemImport 顺序处理题目包：atomic 为真时整组一个事务，任一题失败则全部不入库。
// 非 atomic 时逐个包导入，每个包自身原子，处理一个就更新一次进度。
func (h *Handler) runProblemImport(taskID string, staged []stagedImportFile, u *model.User, bank *model.ProblemBank, maxBytes int64, ip string, atomic bool) {
	ctx := context.Background()
	if atomic {
		items, _ := h.importPackageGroup(ctx, staged, u, bank, maxBytes, ip)
		updateImportTask(taskID, func(task *ImportTask) {
			for _, item := range items {
				task.Done++
				if item.OK {
					task.SuccessCount++
				} else {
					task.FailedCount++
				}
			}
			task.Items = append(task.Items, items...)
		})
	} else {
		for _, file := range staged {
			item := h.importStagedProblem(ctx, file, u, bank, maxBytes, ip)
			updateImportTask(taskID, func(task *ImportTask) {
				task.Done++
				if item.OK {
					task.SuccessCount++
				} else {
					task.FailedCount++
				}
				task.Items = append(task.Items, item)
			})
		}
	}
	finished := time.Now()
	updateImportTask(taskID, func(task *ImportTask) {
		task.Status = importTaskDone
		task.FinishedAt = &finished
	})
	total, success := 0, 0
	if task, ok := snapshotImportTask(taskID); ok {
		total, success = task.Total, task.SuccessCount
	}
	logger.Adminf("管理员 %s 批量导入题目完成：成功 %d / 共 %d ip=%s", u.Username, success, total, ip)
}

// importStagedProblem 从临时文件读取单个题目包并导入；这个包要么整包成功，要么完全不入库。
func (h *Handler) importStagedProblem(ctx context.Context, file stagedImportFile, u *model.User, bank *model.ProblemBank, maxBytes int64, ip string) importItem {
	items, err := h.importPackageGroup(ctx, []stagedImportFile{file}, u, bank, maxBytes, ip)
	if err != nil || len(items) == 0 {
		reason := "导入题目失败"
		if err != nil {
			reason = err.Error()
		}
		return importItem{Filename: file.Name, Error: reason}
	}
	return items[0]
}

// saveStagedFiles 把上传的题目包复制到临时目录，返回文件清单与清理函数。
// 请求结束后 multipart 临时文件会被清理，后台任务必须有自己的副本。
func saveStagedFiles(fileHeaders []*multipart.FileHeader, maxBytes int64) ([]stagedImportFile, func(), error) {
	dir, err := os.MkdirTemp("", "akioi-import-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("创建临时目录失败")
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	staged := make([]stagedImportFile, 0, len(fileHeaders))
	for i, fh := range fileHeaders {
		// 序号前缀避免重名覆盖，原名仅用于结果展示
		dst := filepath.Join(dir, fmt.Sprintf("%04d-%s", i, filepath.Base(fh.Filename)))
		if err := copyUploadedFile(fh, dst, maxBytes); err != nil {
			cleanup()
			return nil, func() {}, err
		}
		staged = append(staged, stagedImportFile{Name: fh.Filename, Path: dst})
	}
	return staged, cleanup, nil
}

// copyUploadedFile 流式复制上传文件，并在超过上限时立即停止。
func copyUploadedFile(fh *multipart.FileHeader, dst string, maxBytes int64) error {
	if maxBytes > 0 && fh.Size > maxBytes {
		return fmt.Errorf("文件 %s 超过大小上限", fh.Filename)
	}
	src, err := fh.Open()
	if err != nil {
		return fmt.Errorf("读取文件 %s 失败", fh.Filename)
	}
	defer func() { _ = src.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("写入临时文件失败")
	}
	defer func() { _ = out.Close() }()
	var reader io.Reader = src
	if maxBytes > 0 {
		reader = io.LimitReader(src, maxBytes+1)
	}
	written, err := io.Copy(out, reader)
	if err != nil {
		return fmt.Errorf("写入临时文件失败")
	}
	if maxBytes > 0 && written > maxBytes {
		return fmt.Errorf("文件 %s 超过大小上限", fh.Filename)
	}
	return nil
}
