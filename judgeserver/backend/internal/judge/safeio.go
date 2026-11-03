package judge

import (
	"errors"
	"io"
	"os"
)

// maxTimeReportBytes 限制 /usr/bin/time 报告的读取上限，避免特殊文件拖垮评测进程。
const maxTimeReportBytes = 64 << 10

var errFileTooLarge = errors.New("文件超过读取上限")

// readHeadSafe 只读取普通文件开头的一段；拒绝符号链接与特殊文件。
// 选手对 /work 有写权限，因此不能直接 os.Open：否则一个指向沙箱外的符号链接
// 就会被评测进程跟随并读回来。
func readHeadSafe(path string, max int64) string {
	f, err := openRegular(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, max))
	if err != nil {
		return ""
	}
	return string(b)
}

// readFileSafe 读取整个普通文件，超过上限时返回错误；同样拒绝符号链接与特殊文件。
func readFileSafe(path string, max int64) ([]byte, error) {
	f, err := openRegular(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > max {
		return nil, errFileTooLarge
	}
	return io.ReadAll(io.LimitReader(f, max))
}

// isRegularFile 判断路径是普通文件、不是符号链接且有内容。
func isRegularFile(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Size() > 0
}
