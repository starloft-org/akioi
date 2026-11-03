//go:build windows

package judge

import (
	"fmt"
	"os"
)

// openRegular 在 Windows 上退回 Lstat 检查（评测节点实际运行在 Linux 容器里，
// 这里只是为了让本仓库能在 Windows 上编译和跑单元测试）。
func openRegular(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("不是普通文件")
	}
	return os.Open(path)
}
