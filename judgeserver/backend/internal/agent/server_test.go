package agent

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("创建 zip 条目失败: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("写入 zip 条目失败: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭 zip 失败: %v", err)
	}
	return buf.Bytes()
}

func TestExtractZipAcceptsPlainNames(t *testing.T) {
	files, err := extractZip(makeZip(t, map[string]string{"1.in": "a", "1.out": "b"}), 1<<20)
	if err != nil {
		t.Fatalf("解压失败: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("期望 2 个文件，得到 %d", len(files))
	}
}

func TestExtractZipRejectsPathEntry(t *testing.T) {
	if _, err := extractZip(makeZip(t, map[string]string{"../evil.in": "x"}), 1<<20); err == nil {
		t.Fatal("带路径的条目应被拒绝")
	}
}

func TestExtractZipRejectsOversize(t *testing.T) {
	if _, err := extractZip(makeZip(t, map[string]string{"1.in": "0123456789"}), 4); err == nil {
		t.Fatal("解压总量超过上限应被拒绝")
	}
}

func TestExtractZipRejectsInvalidPackage(t *testing.T) {
	if _, err := extractZip([]byte("not a zip"), 1<<20); err == nil {
		t.Fatal("非 zip 内容应被拒绝")
	}
}

func TestReplaceDirReplacesWholeDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "1001")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "9.in"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replaceDir(dir, map[string][]byte{"1.in": []byte("new")}); err != nil {
		t.Fatalf("替换失败: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "1.in" {
		t.Fatalf("目录应只剩新文件，实际 %v", entries)
	}
}
