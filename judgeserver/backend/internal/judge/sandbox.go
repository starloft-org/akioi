package judge

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"akioi/internal/config"
)

// RunSpec 描述一次沙箱执行：容器内以 sh -c 执行 ShellCommand。
type RunSpec struct {
	Name             string
	ShellCommand     string
	MemoryMB         int
	CPUs             string
	Timeout          time.Duration
	WorkHostDir      string
	TestdataHostDir  string
	OutputLimitBytes int64
}

// RunResult 是沙箱执行的外部可观测结果。
type RunResult struct {
	ExitCode int
	TimedOut bool
	Stderr   string
	WallMS   int
}

// Sandbox 通过宿主 docker 创建一次性容器执行不可信代码。
type Sandbox struct {
	cfg *config.JudgeConfig
}

func NewSandbox(cfg *config.JudgeConfig) *Sandbox {
	return &Sandbox{cfg: cfg}
}

// Run 启动沙箱容器并在超时或结束后清理；容器内网络、根文件系统与能力均受限。
func (s *Sandbox) Run(ctx context.Context, spec RunSpec) (RunResult, error) {
	if spec.Name == "" {
		return RunResult{}, fmt.Errorf("沙箱任务名称为空")
	}
	args := []string{
		"run", "--rm", "--name", spec.Name,
		"--network", "none",
		"--read-only",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--pids-limit", "64",
		"--memory", strconv.Itoa(spec.MemoryMB) + "m",
		"--memory-swap", strconv.Itoa(spec.MemoryMB) + "m",
		"--cpus", spec.CPUs,
		"--tmpfs", fmt.Sprintf("/tmp:rw,size=%dm,mode=1777", s.cfg.TmpfsMB),
		"-v", spec.WorkHostDir + ":/work:rw",
		"-w", "/work",
	}
	if spec.OutputLimitBytes > 0 {
		limit := strconv.FormatInt(spec.OutputLimitBytes, 10)
		args = append(args, "--ulimit", "fsize="+limit+":"+limit)
	}
	if spec.TestdataHostDir != "" {
		args = append(args, "-v", spec.TestdataHostDir+":/box:ro")
	}
	args = append(args, s.cfg.SandboxImage, "sh", "-c", spec.ShellCommand)

	cmd := exec.CommandContext(ctx, s.cfg.DockerBin, args...)
	stderr := &cappedBuffer{max: 64 << 10}
	cmd.Stderr = stderr

	start := time.Now()
	err := cmd.Run()
	wall := time.Since(start)

	result := RunResult{
		ExitCode: -1,
		Stderr:   truncate(stderr.String(), 4000),
		WallMS:   int(wall.Milliseconds()),
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() == context.DeadlineExceeded || (err != nil && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() < 0) {
		result.TimedOut = true
	}
	if result.TimedOut {
		s.kill(spec.Name)
	}
	if err != nil && !result.TimedOut {
		// 非零退出码由上层按状态判读，此处只保留错误文本。
		result.Stderr = truncate(strings.TrimSpace(result.Stderr+" "+err.Error()), 4000)
	}
	return result, nil
}

// kill 强制清理超时后可能仍存活的容器。
func (s *Sandbox) kill(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, s.cfg.DockerBin, "kill", name).Run()
	_ = exec.CommandContext(ctx, s.cfg.DockerBin, "rm", "-f", name).Run()
}

// CheckAvailable 检查 docker 可执行文件与沙箱镜像是否存在，供启动自检使用。
func (s *Sandbox) CheckAvailable(ctx context.Context) error {
	if _, err := exec.LookPath(s.cfg.DockerBin); err != nil {
		return fmt.Errorf("未找到 docker 可执行文件 %s: %w", s.cfg.DockerBin, err)
	}
	out, err := exec.CommandContext(ctx, s.cfg.DockerBin, "image", "inspect", s.cfg.SandboxImage).CombinedOutput()
	if err != nil {
		return fmt.Errorf("沙箱镜像 %s 不可用: %s", s.cfg.SandboxImage, strings.TrimSpace(string(out)))
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...(已截断)"
}

// cappedBuffer 只保留开头的固定字节数，超出部分继续消费但直接丢弃，
// 避免 docker 子进程的输出把评测服务内存撑大或阻塞管道。
type cappedBuffer struct {
	buf bytes.Buffer
	max int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if c.max > 0 && c.buf.Len() < c.max {
		room := c.max - c.buf.Len()
		if len(p) <= room {
			_, _ = c.buf.Write(p)
		} else {
			_, _ = c.buf.Write(p[:room])
		}
	}
	return len(p), nil
}

func (c *cappedBuffer) String() string { return c.buf.String() }
