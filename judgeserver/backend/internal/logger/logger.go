package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var (
	judgeLog    *log.Logger
	businessLog *log.Logger
	errorLog    *log.Logger
)

// rotatingWriter 按大小轮转文件，历史文件以递增序号永久保留。
type rotatingWriter struct {
	mu       sync.Mutex
	dir      string
	name     string
	maxBytes int64
	file     *os.File
	size     int64
}

func newRotatingWriter(dir, name string, maxBytes int64) (*rotatingWriter, error) {
	w := &rotatingWriter{dir: dir, name: name, maxBytes: maxBytes}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) open() error {
	path := filepath.Join(w.dir, w.name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file = f
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.maxBytes > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	next := w.nextIndex()
	src := filepath.Join(w.dir, w.name)
	dst := fmt.Sprintf("%s.%d", src, next)
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	return w.open()
}

// nextIndex 扫描现存备份取最大序号 +1，保证序号只增不减。
func (w *rotatingWriter) nextIndex() int {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return 1
	}
	prefix := w.name + "."
	max := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(e.Name(), prefix))
		if err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// Init 初始化评测日志；dir 为空时只输出到控制台。
func Init(dir string, maxMB int) error {
	maxBytes := int64(maxMB) * 1024 * 1024

	stdout := io.Writer(os.Stdout)
	stderr := io.Writer(os.Stderr)

	fileWriter := func(name string) (io.Writer, error) {
		if dir == "" {
			return nil, nil
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		return newRotatingWriter(dir, name, maxBytes)
	}

	newLogger := func(name string, console io.Writer) (*log.Logger, error) {
		fw, err := fileWriter(name)
		if err != nil {
			return nil, err
		}
		w := console
		if fw != nil {
			w = io.MultiWriter(fw, console)
		}
		return log.New(w, "", log.LstdFlags|log.Lmicroseconds), nil
	}

	var err error
	if judgeLog, err = newLogger("judge.log", stdout); err != nil {
		return err
	}
	if businessLog, err = newLogger("business.log", stdout); err != nil {
		return err
	}
	if errorLog, err = newLogger("error.log", stderr); err != nil {
		return err
	}
	return nil
}

func Judgef(format string, args ...any)    { logf(judgeLog, format, args...) }
func Businessf(format string, args ...any) { logf(businessLog, format, args...) }

func Errorf(format string, args ...any) {
	if errorLog == nil {
		return
	}
	errorLog.Printf(format, args...)
}

func logf(l *log.Logger, format string, args ...any) {
	if l == nil {
		return
	}
	l.Printf(format, args...)
}
