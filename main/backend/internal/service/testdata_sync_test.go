package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"akioi/internal/config"
)

func newSyncTestService(t *testing.T, endpoints ...string) *TestdataService {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1", "1.in"), []byte("in"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Judge: config.JudgeConfig{
		TestdataDir: dir,
		SyncSecret:  "test-secret",
	}}
	settings := DefaultRuntimeSettings()
	settings.JudgeNodeEndpoints = endpoints
	settings.JudgeSyncTimeoutSec = 5
	return NewTestdata(nil, cfg, NewStaticSettingService(settings))
}

func TestSyncToNodesPushesInOrder(t *testing.T) {
	var mu sync.Mutex
	var order []string
	handler := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Sync-Secret") != "test-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.Method != http.MethodPut || r.URL.Path != "/internal/v1/testdata/1" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if _, err := io.Copy(io.Discard, r.Body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}
	}
	first := httptest.NewServer(handler("first"))
	defer first.Close()
	second := httptest.NewServer(handler("second"))
	defer second.Close()

	// 第一个地址带结尾斜杠，验证地址归一化
	svc := newSyncTestService(t, first.URL+"/", second.URL)
	if failed := svc.SyncToNodes(context.Background(), 1); len(failed) != 0 {
		t.Fatalf("不应有失败节点: %v", failed)
	}
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("应按配置顺序依次推送，实际 %v", order)
	}
}

func TestSyncToNodesReportsFailedNode(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer good.Close()

	svc := newSyncTestService(t, bad.URL, good.URL)
	failed := svc.SyncToNodes(context.Background(), 1)
	if len(failed) != 1 || failed[0] != bad.URL {
		t.Fatalf("应只报告失败节点，实际 %v", failed)
	}
}

func TestClearOnNodesUsesDelete(t *testing.T) {
	var mu sync.Mutex
	method := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		method = r.Method
		mu.Unlock()
	}))
	defer srv.Close()

	svc := newSyncTestService(t, srv.URL)
	if failed := svc.ClearOnNodes(context.Background(), 1); len(failed) != 0 {
		t.Fatalf("不应有失败节点: %v", failed)
	}
	if method != http.MethodDelete {
		t.Fatalf("期望 DELETE，实际 %s", method)
	}
}

func TestSyncToNodesWithoutEndpoints(t *testing.T) {
	svc := newSyncTestService(t)
	if failed := svc.SyncToNodes(context.Background(), 1); failed != nil {
		t.Fatalf("未配置节点时不应有失败节点: %v", failed)
	}
}
