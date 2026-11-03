//go:build !windows

package judge

import (
	"fmt"
	"os"
	"syscall"
)

// openRegular 以 O_NOFOLLOW 打开文件，并核验打开后的描述符确实是普通文件。
// 只做 Lstat 再 open 仍存在竞态，O_NOFOLLOW 让内核直接拒绝最后一跳是符号链接的情况。
func openRegular(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("不是普通文件")
	}
	return f, nil
}
