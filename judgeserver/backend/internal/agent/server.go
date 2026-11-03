// Package agent 是评测节点的控制面接口：接收主站推送的测试数据。
package agent

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"akioi/internal/config"
	"akioi/internal/logger"
	"akioi/internal/model"
	"akioi/internal/queue"

	"gorm.io/gorm"
)

// Server 监听主站推送的测试数据与评测任务。
type Server struct {
	cfg   *config.Config
	db    *gorm.DB
	queue *queue.Queue
	srv   *http.Server
}

func New(cfg *config.Config, db *gorm.DB, q *queue.Queue) *Server {
	s := &Server{cfg: cfg, db: db, queue: q}
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /internal/v1/testdata/{id}", s.withSecret(s.putTestdata))
	mux.HandleFunc("DELETE /internal/v1/testdata/{id}", s.withSecret(s.deleteTestdata))
	mux.HandleFunc("POST /internal/v1/judge", s.withSecret(s.postJudge))
	mux.HandleFunc("GET /internal/v1/health", s.withSecret(s.health))
	s.srv = &http.Server{
		Addr:              cfg.Judge.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

// Run 启动监听，ctx 结束后优雅关闭。
func (s *Server) Run(ctx context.Context) {
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutdownCtx)
	}()
	logger.Businessf("测试数据接收接口监听 %s", s.cfg.Judge.ListenAddr)
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Errorf("测试数据接收接口退出: %v", err)
	}
}

// withSecret 校验主站同步密钥，未配置或密钥不符时拒绝请求。
func (s *Server) withSecret(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		given := r.Header.Get("X-Sync-Secret")
		if s.cfg.Judge.SyncSecret == "" ||
			subtle.ConstantTimeCompare([]byte(given), []byte(s.cfg.Judge.SyncSecret)) != 1 {
			writeJSON(w, http.StatusUnauthorized, "同步密钥不正确")
			return
		}
		next(w, r)
	}
}

// putTestdata 用请求体中的数据包整体替换某题的测试数据。
// postJudge 接收主站下发的评测任务：写入本地任务表并推入队列。
func (s *Server) postJudge(w http.ResponseWriter, r *http.Request) {
	var task model.JudgeTask
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&task); err != nil {
		writeJSON(w, http.StatusBadRequest, "评测任务格式错误")
		return
	}
	if task.SubmissionID == 0 || task.JudgeToken == "" {
		writeJSON(w, http.StatusBadRequest, "评测任务缺少必要字段")
		return
	}
	payload, err := json.Marshal(task)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "序列化任务失败")
		return
	}

	job := model.JudgeJob{
		SubmissionID: task.SubmissionID,
		JudgeToken:   task.JudgeToken,
		Payload:      string(payload),
		Status:       "pending",
	}

	var existing model.JudgeJob
	err = s.db.Where("submission_id = ?", task.SubmissionID).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if err := s.db.Create(&job).Error; err != nil {
			logger.Errorf("保存评测任务失败 submission=%d: %v", task.SubmissionID, err)
			writeJSON(w, http.StatusInternalServerError, "保存任务失败")
			return
		}
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, "查询任务失败")
		return
	default:
		if existing.JudgeToken == task.JudgeToken && existing.Status != "failed" {
			// 同一任务重复下发，已经在队列或正在评测
			writeJSON(w, http.StatusOK, "")
			return
		}
		existing.JudgeToken = task.JudgeToken
		existing.Payload = string(payload)
		existing.Status = "pending"
		existing.Result = ""
		existing.Synced = false
		if err := s.db.Save(&existing).Error; err != nil {
			writeJSON(w, http.StatusInternalServerError, "更新任务失败")
			return
		}
		job = existing
	}

	if err := s.queue.Push(r.Context(), queue.Task{JobID: job.ID}); err != nil {
		logger.Errorf("投递本地队列失败 job=%d: %v", job.ID, err)
		writeJSON(w, http.StatusInternalServerError, "任务入队失败")
		return
	}
	logger.Judgef("已接收评测任务 submission=%d job=%d", task.SubmissionID, job.ID)
	writeJSON(w, http.StatusOK, "")
}

// health 返回节点运行概况，供主站的测评机管理页展示。
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	queueLen, _ := s.queue.Length(r.Context())
	var pending int64
	_ = s.db.Model(&model.JudgeJob{}).
		Where("status IN ?", []string{"pending", "running"}).
		Count(&pending).Error
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":            true,
		"queue_length":  queueLen,
		"pending_jobs":  pending,
		"data_problems": countDataProblems(s.cfg.Judge.TestdataDir),
	})
}

// countDataProblems 统计测试数据目录下以数字命名的题目目录数量。
func countDataProblems(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err == nil {
			count++
		}
	}
	return count
}

func (s *Server) putTestdata(w http.ResponseWriter, r *http.Request) {
	problemID, ok := pathProblemID(w, r)
	if !ok {
		return
	}
	maxBytes := int64(s.cfg.Judge.MaxTestdataMB) * 1024 * 1024
	content, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBytes))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "读取数据包失败或超过大小上限")
		return
	}
	files, err := extractZip(content, maxBytes)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, "数据包内没有测试数据文件")
		return
	}
	if err := replaceDir(s.problemDir(problemID), files); err != nil {
		logger.Errorf("写入题目 %d 测试数据失败: %v", problemID, err)
		writeJSON(w, http.StatusInternalServerError, "写入测试数据失败")
		return
	}
	logger.Judgef("已接收题目 %d 测试数据，共 %d 个文件", problemID, len(files))
	writeJSON(w, http.StatusOK, "")
}

// deleteTestdata 删除某题在本机的测试数据。
func (s *Server) deleteTestdata(w http.ResponseWriter, r *http.Request) {
	problemID, ok := pathProblemID(w, r)
	if !ok {
		return
	}
	if err := os.RemoveAll(s.problemDir(problemID)); err != nil {
		logger.Errorf("删除题目 %d 测试数据失败: %v", problemID, err)
		writeJSON(w, http.StatusInternalServerError, "删除测试数据失败")
		return
	}
	logger.Judgef("已删除题目 %d 的本机测试数据", problemID)
	writeJSON(w, http.StatusOK, "")
}

func (s *Server) problemDir(problemID uint) string {
	return filepath.Join(s.cfg.Judge.TestdataDir, strconv.FormatUint(uint64(problemID), 10))
}

func pathProblemID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || id == 0 {
		writeJSON(w, http.StatusBadRequest, "题目 ID 不合法")
		return 0, false
	}
	return uint(id), true
}

// extractZip 展开数据包；只接受纯文件名，解压总量不得超过上限。
// extractZip 展开数据包；只接受纯文件名，解压总量与文件数都在读取过程中受限。
func extractZip(content []byte, maxBytes int64) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, errors.New("数据包不是合法的 zip 文件")
	}
	if len(zr.File) > maxZipEntries {
		return nil, errors.New("数据包内文件数超过上限")
	}
	out := make(map[string][]byte, len(zr.File))
	var total int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		if name != filepath.Base(name) || strings.HasPrefix(name, ".") {
			return nil, errors.New("数据包内含非法文件名: " + name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		remaining := maxBytes - total
		if remaining < 0 {
			_ = rc.Close()
			return nil, errors.New("数据包解压后超过大小上限")
		}
		buf, err := io.ReadAll(io.LimitReader(rc, remaining+1))
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		if int64(len(buf)) > remaining {
			return nil, errors.New("数据包解压后超过大小上限")
		}
		total += int64(len(buf))
		out[name] = buf
	}
	return out, nil
}

const maxZipEntries = 20000

// replaceDir 用新文件整体替换目录；先写临时目录再改名，失败不会留下半个数据包。
func replaceDir(dir string, files map[string][]byte) error {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, filepath.Base(dir)+".tmp-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmp, name), content, 0o644); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.Rename(tmp, dir)
}

func writeJSON(w http.ResponseWriter, status int, message string) {
	code := 0
	if status != http.StatusOK {
		code = 1
	}
	if message == "" {
		message = "ok"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "message": message})
}
