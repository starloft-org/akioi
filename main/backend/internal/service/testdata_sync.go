package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"akioi/internal/logger"
)

// SyncToNodes 依次把某题的测试数据推送到各评测节点，返回推送失败的节点。
func (s *TestdataService) SyncToNodes(ctx context.Context, problemID uint) []string {
	nodes := s.settings.Current().JudgeNodeEndpoints
	if len(nodes) == 0 {
		return nil
	}
	zipPath, err := s.packageFile(problemID)
	if err != nil {
		logger.Errorf("打包题目 %d 测试数据失败: %v", problemID, err)
		return append([]string{}, nodes...)
	}
	defer func() { _ = os.Remove(zipPath) }()

	return s.eachNode(ctx, problemID, func(ctx context.Context, endpoint string) error {
		return s.pushPackage(ctx, endpoint, problemID, zipPath)
	})
}

// ClearOnNodes 依次通知各评测节点删除某题的测试数据，返回通知失败的节点。
func (s *TestdataService) ClearOnNodes(ctx context.Context, problemID uint) []string {
	if len(s.settings.Current().JudgeNodeEndpoints) == 0 {
		return nil
	}
	return s.eachNode(ctx, problemID, func(ctx context.Context, endpoint string) error {
		return s.deleteOnNode(ctx, endpoint, problemID)
	})
}

// eachNode 按配置顺序逐个节点执行，单个节点失败只记录并继续。
func (s *TestdataService) eachNode(ctx context.Context, problemID uint,
	action func(ctx context.Context, endpoint string) error) []string {

	var failed []string
	for _, endpoint := range s.settings.Current().JudgeNodeEndpoints {
		if err := action(ctx, endpoint); err != nil {
			logger.Errorf("同步题目 %d 测试数据到评测节点 %s 失败: %v", problemID, endpoint, err)
			failed = append(failed, endpoint)
			continue
		}
		logger.Businessf("题目 %d 测试数据已同步到评测节点 %s", problemID, endpoint)
	}
	return failed
}

// pushPackage 把数据包上传到节点替换该题测试数据；请求体直接读临时文件，不整包进内存。
func (s *TestdataService) pushPackage(ctx context.Context, endpoint string, problemID uint, zipPath string) error {
	f, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.nodeURL(endpoint, problemID), f)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("X-Sync-Secret", s.cfg.Judge.SyncSecret)
	return s.do(req)
}

// deleteOnNode 通知节点删除该题在本机的测试数据。
func (s *TestdataService) deleteOnNode(ctx context.Context, endpoint string, problemID uint) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.nodeURL(endpoint, problemID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Sync-Secret", s.cfg.Judge.SyncSecret)
	return s.do(req)
}

func (s *TestdataService) do(req *http.Request) error {
	client := &http.Client{Timeout: time.Duration(s.settings.Current().JudgeSyncTimeoutSec) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("节点返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (s *TestdataService) nodeURL(endpoint string, problemID uint) string {
	return strings.TrimRight(endpoint, "/") + "/internal/v1/testdata/" + strconv.FormatUint(uint64(problemID), 10)
}

// packageFile 把某题测试数据打包到临时文件，避免大包占用内存。
func (s *TestdataService) packageFile(problemID uint) (string, error) {
	f, err := os.CreateTemp("", "akioi-testdata-*.zip")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := writeZip(f, s.Dir(problemID)); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}
