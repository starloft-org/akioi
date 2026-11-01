package handler

import (
	"bytes"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestImportTaskLifecycle 覆盖任务登记、进度更新、快照与状态收尾。
func TestImportTaskLifecycle(t *testing.T) {
	task := createImportTask(3)
	if task.ID == "" || task.Total != 3 || task.Status != importTaskRunning {
		t.Fatalf("unexpected task: %+v", task)
	}

	updateImportTask(task.ID, func(current *ImportTask) {
		current.Done = 1
		current.SuccessCount = 1
		current.Items = append(current.Items, importItem{Filename: "a.zip", OK: true})
	})

	snapshot, ok := snapshotImportTask(task.ID)
	if !ok {
		t.Fatal("task should exist")
	}
	if snapshot.Done != 1 || snapshot.SuccessCount != 1 || len(snapshot.Items) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}

	finished := time.Now()
	updateImportTask(task.ID, func(current *ImportTask) {
		current.Status = importTaskDone
		current.FinishedAt = &finished
	})
	snapshot, _ = snapshotImportTask(task.ID)
	if snapshot.Status != importTaskDone || snapshot.FinishedAt == nil {
		t.Fatalf("task should be done: %+v", snapshot)
	}

	if _, ok := snapshotImportTask("missing"); ok {
		t.Fatal("unknown task should not be found")
	}
}

// TestImportTaskGC 清理过期任务，避免内存里长期保留大量导入结果。
func TestImportTaskGC(t *testing.T) {
	task := createImportTask(1)
	old := time.Now().Add(-2 * time.Hour)
	updateImportTask(task.ID, func(current *ImportTask) {
		current.Status = importTaskDone
		current.FinishedAt = &old
	})

	importTasksMu.Lock()
	gcImportTasksLocked(time.Now())
	_, exists := importTasks[task.ID]
	importTasksMu.Unlock()
	if exists {
		t.Fatal("expired task should be collected")
	}
}

// TestCopyUploadedFile 校验流式复制与大小上限。
func TestCopyUploadedFile(t *testing.T) {
	header := newTestUpload(t, "1.zip", "hello world")
	dir := t.TempDir()
	dst := filepath.Join(dir, "copy.zip")
	if err := copyUploadedFile(header, dst, 0); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	content, err := os.ReadFile(dst)
	if err != nil || string(content) != "hello world" {
		t.Fatalf("unexpected content: %q err=%v", content, err)
	}

	if err := copyUploadedFile(header, filepath.Join(dir, "too-big.zip"), 3); err == nil {
		t.Fatal("oversize file should be rejected")
	} else if !strings.Contains(err.Error(), "超过大小上限") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSaveStagedFiles 校验题目包落到临时目录并能被清理。
func TestSaveStagedFiles(t *testing.T) {
	headers := []*multipart.FileHeader{
		newTestUpload(t, "a.zip", "aaa"),
		newTestUpload(t, "a.zip", "bbb"),
	}
	staged, cleanup, err := saveStagedFiles(headers, 0)
	if err != nil {
		t.Fatalf("save staged files failed: %v", err)
	}
	if len(staged) != 2 {
		t.Fatalf("want 2 staged files, got %d", len(staged))
	}
	// 重名文件用序号前缀区分，不会互相覆盖
	if staged[0].Path == staged[1].Path {
		t.Fatal("staged paths should differ")
	}
	for i, want := range []string{"aaa", "bbb"} {
		content, err := os.ReadFile(staged[i].Path)
		if err != nil || string(content) != want {
			t.Fatalf("file %d content mismatch: %q err=%v", i, content, err)
		}
	}
	dir := filepath.Dir(staged[0].Path)
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temp dir should be removed, stat err=%v", err)
	}
}

// newTestUpload 构造一个内存中的上传文件头。
func newTestUpload(t *testing.T, name, content string) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("files", name)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	reader := multipart.NewReader(body, writer.Boundary())
	form, err := reader.ReadForm(int64(body.Len()) + 1024)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	if len(form.File["files"]) == 0 {
		t.Fatal("no uploaded file parsed")
	}
	return form.File["files"][0]
}
