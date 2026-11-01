package handler

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// importArchiveMaxFiles 限制一次展开出的题目包数量，避免远超数据库承受范围。
	importArchiveMaxFiles = 5000
	// importArchiveMaxDepth 限制外层 zip 的嵌套层数。
	importArchiveMaxDepth = 2
	// importArchiveMaxTotal 限制展开后的总字节数，防止压缩炸弹写满磁盘。
	importArchiveMaxTotal = int64(8) << 30
)

// importExpander 把「多个题目包打成一个外层 zip」的上传展开成一个个题目包。
type importExpander struct {
	maxBytes  int64
	dirs      []string
	totalSize int64
	count     int
}

// expandImportContainers 展开上传文件里包含的内层 zip。
// 只要有一个文件被识别成打包文件（内部有 zip 且没有 problem.md），就返回展开后的列表。
func expandImportContainers(staged []stagedImportFile, maxBytes int64) ([]stagedImportFile, bool, func(), error) {
	expander := &importExpander{maxBytes: maxBytes}
	cleanup := func() {
		for _, dir := range expander.dirs {
			_ = os.RemoveAll(dir)
		}
	}

	var result []stagedImportFile
	container := false
	for _, file := range staged {
		files, isContainer, err := expander.expand(file, 1)
		if err != nil {
			cleanup()
			return nil, false, func() {}, err
		}
		if isContainer {
			container = true
		}
		result = append(result, files...)
	}
	if len(result) == 0 {
		return staged, false, cleanup, nil
	}
	return result, container, cleanup, nil
}

// expand 递归展开单个上传文件，嵌套过深的打包文件按普通题目包保留。
func (e *importExpander) expand(file stagedImportFile, depth int) ([]stagedImportFile, bool, error) {
	inner, dir, isContainer, err := e.readArchive(file, depth)
	if err != nil {
		return nil, false, err
	}
	if !isContainer {
		return []stagedImportFile{file}, false, nil
	}
	e.dirs = append(e.dirs, dir)
	var result []stagedImportFile
	for _, item := range inner {
		files, _, err := e.expand(item, depth+1)
		if err != nil {
			return nil, false, err
		}
		result = append(result, files...)
	}
	return result, true, nil
}

// readArchive 找出压缩包里的题目包：内部含 problem.md 的按普通题目包处理，
// 否则把包内的 .zip 逐个解到临时目录，作为题目包列表返回。
func (e *importExpander) readArchive(file stagedImportFile, depth int) ([]stagedImportFile, string, bool, error) {
	reader, err := zip.OpenReader(file.Path)
	if err != nil {
		// 不是 zip 时交给后续解析报错，保持原有提示
		return nil, "", false, nil
	}
	defer func() { _ = reader.Close() }()

	var entries []*zip.File
	hasManifest := false
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		base := strings.ToLower(filepath.Base(name))
		if strings.Contains(name, "__MACOSX/") || strings.HasPrefix(base, ".") {
			continue
		}
		if base == "problem.md" {
			hasManifest = true
			break
		}
		if strings.HasSuffix(base, ".zip") {
			entries = append(entries, entry)
		}
	}
	if hasManifest || len(entries) == 0 {
		return nil, "", false, nil
	}
	if depth > importArchiveMaxDepth {
		return nil, "", false, fmt.Errorf("题目包嵌套层级过深，请先解开外层 zip 再导入")
	}

	dir, err := os.MkdirTemp("", "akioi-import-archive-*")
	if err != nil {
		return nil, "", false, fmt.Errorf("创建临时目录失败: %v", err)
	}
	result := make([]stagedImportFile, 0, len(entries))
	for i, entry := range entries {
		e.count++
		if e.count > importArchiveMaxFiles {
			_ = os.RemoveAll(dir)
			return nil, "", false, fmt.Errorf("一次最多导入 %d 个题目包", importArchiveMaxFiles)
		}
		name := filepath.Base(strings.ReplaceAll(entry.Name, "\\", "/"))
		dst := filepath.Join(dir, fmt.Sprintf("%04d-%s", i+1, name))
		if err := e.writeEntry(entry, dst); err != nil {
			_ = os.RemoveAll(dir)
			return nil, "", false, err
		}
		result = append(result, stagedImportFile{Name: name, Path: dst})
	}
	return result, dir, true, nil
}

// writeEntry 把压缩包内的条目写到临时文件，写入过程中限制大小与总占用。
func (e *importExpander) writeEntry(entry *zip.File, dst string) error {
	if size := int64(entry.UncompressedSize64); size > e.maxBytes {
		return fmt.Errorf("包内文件 %s 超过大小上限", entry.Name)
	}
	src, err := entry.Open()
	if err != nil {
		return fmt.Errorf("读取包内文件 %s 失败: %v", entry.Name, err)
	}
	defer func() { _ = src.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("写入包内文件 %s 失败: %v", entry.Name, err)
	}
	written, copyErr := io.Copy(out, io.LimitReader(src, e.maxBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("解压包内文件 %s 失败: %v", entry.Name, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("写入包内文件 %s 失败: %v", entry.Name, closeErr)
	}
	if written > e.maxBytes {
		_ = os.Remove(dst)
		return fmt.Errorf("包内文件 %s 超过大小上限", entry.Name)
	}
	e.totalSize += written
	if e.totalSize > importArchiveMaxTotal {
		_ = os.Remove(dst)
		return fmt.Errorf("压缩包解压后超过 %d GB，请分批导入", importArchiveMaxTotal>>30)
	}
	return nil
}
