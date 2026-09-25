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
)

// Server 监听主站推送测试数据的请求。
type Server struct {
	cfg *config.Config
	srv *http.Server
}

func New(cfg *config.Config) *Server {
	s := &Server{cfg: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /internal/v1/testdata/{id}", s.withSecret(s.putTestdata))
	mux.HandleFunc("DELETE /internal/v1/testdata/{id}", s.withSecret(s.deleteTestdata))
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
func extractZip(content []byte, maxBytes int64) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, errors.New("数据包不是合法的 zip 文件")
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
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(rc); err != nil {
			_ = rc.Close()
			return nil, err
		}
		_ = rc.Close()
		total += int64(buf.Len())
		if total > maxBytes {
			return nil, errors.New("数据包解压后超过大小上限")
		}
		out[name] = buf.Bytes()
	}
	return out, nil
}

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
