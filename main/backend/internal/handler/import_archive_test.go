package handler

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// makeZip 在指定路径写入压缩包，entries 的 value 为 nil 时表示目录。
func makeZip(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	for name, content := range entries {
		if content == nil {
			if _, err := writer.Create(name + "/"); err != nil {
				t.Fatalf("create dir entry: %v", err)
			}
			continue
		}
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
}

// TestExpandImportContainersPacked 外层 zip 里装多个题目包时展开成多个包。
func TestExpandImportContainersPacked(t *testing.T) {
	dir := t.TempDir()
	makeZip(t, filepath.Join(dir, "a.zip"), map[string][]byte{
		"problem.md": []byte("# A\n"),
		"1.in":       []byte("1\n"),
	})
	makeZip(t, filepath.Join(dir, "b.zip"), map[string][]byte{
		"problem.md": []byte("# B\n"),
		"1.in":       []byte("2\n"),
	})
	packed := filepath.Join(dir, "all.zip")
	makeZip(t, packed, map[string][]byte{
		"round1/":      nil,
		"round1/a.zip": mustRead(t, filepath.Join(dir, "a.zip")),
		"round1/b.zip": mustRead(t, filepath.Join(dir, "b.zip")),
		"readme.txt":   []byte("ignore me"),
	})

	staged := []stagedImportFile{{Name: "all.zip", Path: packed}}
	files, container, cleanup, err := expandImportContainers(staged, 1<<20)
	if err != nil {
		t.Fatalf("expand failed: %v", err)
	}
	defer cleanup()
	if !container {
		t.Fatal("should be treated as container")
	}
	if len(files) != 2 {
		t.Fatalf("want 2 packages, got %d", len(files))
	}
	for _, file := range files {
		if _, err := os.Stat(file.Path); err != nil {
			t.Fatalf("extracted file missing: %v", err)
		}
	}
	extracted := filepath.Dir(files[0].Path)
	cleanup()
	if _, err := os.Stat(extracted); !os.IsNotExist(err) {
		t.Fatalf("temp dir should be removed, err=%v", err)
	}
}

// TestExpandImportContainersPlainPackage 单个题目包（含 problem.md）保持原样。
func TestExpandImportContainersPlainPackage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "single.zip")
	makeZip(t, path, map[string][]byte{
		"problem.md": []byte("# A\n"),
		"1.in":       []byte("1\n"),
	})

	files, container, cleanup, err := expandImportContainers([]stagedImportFile{{Name: "single.zip", Path: path}}, 1<<20)
	if err != nil {
		t.Fatalf("expand failed: %v", err)
	}
	defer cleanup()
	if container {
		t.Fatal("single package should not be treated as container")
	}
	if len(files) != 1 || files[0].Path != path {
		t.Fatalf("plain package should be kept as is: %+v", files)
	}
}

// TestExpandImportContainersTooMany 超过数量上限时报错。
func TestExpandImportContainersTooMany(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "inner.zip")
	makeZip(t, inner, map[string][]byte{"problem.md": []byte("# A\n")})
	content := mustRead(t, inner)

	entries := map[string][]byte{}
	for i := 0; i < importArchiveMaxFiles+1; i++ {
		entries[filepath.Join("batch", "p"+itoa(i)+".zip")] = content
	}
	packed := filepath.Join(dir, "huge.zip")
	makeZip(t, packed, entries)

	_, _, _, err := expandImportContainers([]stagedImportFile{{Name: "huge.zip", Path: packed}}, 1<<20)
	if err == nil {
		t.Fatal("should reject too many packages")
	}
}

// TestExpandImportContainersRejectsOversize 包内单个题目包超过上限时报错。
func TestExpandImportContainersRejectsOversize(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "big.zip")
	// 上限按题目包 zip 文件本身的大小计算，这里把上限压到 64 字节
	makeZip(t, inner, map[string][]byte{"problem.md": make([]byte, 4096)})
	packed := filepath.Join(dir, "all.zip")
	makeZip(t, packed, map[string][]byte{"big.zip": mustRead(t, inner)})

	_, _, _, err := expandImportContainers([]stagedImportFile{{Name: "all.zip", Path: packed}}, 64)
	if err == nil {
		t.Fatal("should reject oversize inner package")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
