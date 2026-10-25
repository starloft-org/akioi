package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"akioi/internal/model"
)

func TestExitDescription(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{139, "段错误"},
		{134, "SIGABRT"},
		{136, "SIGFPE"},
		{132, "SIGILL"},
		{143, "SIGTERM"},
		{200, "信号 72"},
		{2, "退出码 2"},
		{0, "正常结束"},
	}
	for _, tc := range cases {
		got := exitDescription(tc.code)
		if !strings.Contains(got, tc.want) {
			t.Errorf("exitDescription(%d) = %q，期望包含 %q", tc.code, got, tc.want)
		}
	}
}

func TestRuntimeMessage(t *testing.T) {
	if got := runtimeMessage(2); got != "运行时错误，退出码 2" {
		t.Errorf("runtimeMessage(2) = %q", got)
	}
	if got := runtimeMessage(139); !strings.Contains(got, "段错误") {
		t.Errorf("runtimeMessage(139) = %q，期望包含段错误", got)
	}
}

func TestEnvironmentMessage(t *testing.T) {
	if got := environmentMessage(exitCodeTestdataMissing, ""); !strings.Contains(got, "评测数据缺失") {
		t.Errorf("测试数据缺失的说明不清楚: %q", got)
	}
	if got := environmentMessage(exitCodeTimeMissing, ""); !strings.Contains(got, "/usr/bin/time") {
		t.Errorf("缺少 time 的说明不清楚: %q", got)
	}
	if got := environmentMessage(127, ""); !strings.Contains(got, "无法执行选手程序") {
		t.Errorf("退出码 127 的说明不清楚: %q", got)
	}
	// stderr 的首行要作为补充信息带进说明
	got := environmentMessage(2, "cannot create /work/case1.out: Read-only file system\n")
	if !strings.Contains(got, "Read-only file system") {
		t.Errorf("未带上 stderr 的关键信息: %q", got)
	}
	// 前置检查已有明确原因时不再重复拼接 stderr
	dup := environmentMessage(exitCodeTestdataMissing, "测试数据缺失：沙箱内找不到 /box/1.in")
	if dup != environmentBaseMessage(exitCodeTestdataMissing) {
		t.Errorf("前置检查的说明被重复拼接: %q", dup)
	}
}

func TestRunShellChecksPreconditions(t *testing.T) {
	shell := runShell("/work/main", 7, "case7.time", "case7.out", "case7.err")
	for _, want := range []string{
		"/box/7.in",
		"/work/main",
		"/work/case7.time",
		"/work/case7.out",
		"/work/case7.err",
		"exit 97",
		"exit 98",
		"exit 99",
		"/usr/bin/time -v",
	} {
		if !strings.Contains(shell, want) {
			t.Errorf("runShell 缺少 %q:\n%s", want, shell)
		}
	}
	// 前置检查必须排在真正的执行命令之前，否则重定向失败时拿不到原因。
	if strings.Index(shell, "/box/7.in") > strings.Index(shell, "/usr/bin/time -v") {
		t.Errorf("前置检查应在执行命令之前:\n%s", shell)
	}
}

func TestCleanStderrDropsTimeNoise(t *testing.T) {
	raw := "Command exited with non-zero status 2\nterminate called after throwing\nCommand terminated by signal 6\n"
	got := cleanStderr(raw)
	if strings.Contains(got, "Command exited") || strings.Contains(got, "Command terminated") {
		t.Errorf("未清理 time 的状态行: %q", got)
	}
	if !strings.Contains(got, "terminate called after throwing") {
		t.Errorf("误删了选手输出: %q", got)
	}
}

func TestMergeStderrKeepsBothSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case1.err")
	if err := os.WriteFile(path, []byte("user stderr\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := mergeStderr(path, "/bin/sh: cannot open /box/1.in")
	if !strings.Contains(got, "user stderr") || !strings.Contains(got, "/box/1.in") {
		t.Errorf("合并 stderr 失败: %q", got)
	}
}

func TestSystemMessagePrefersFirstSystemError(t *testing.T) {
	results := []model.TestcaseResult{
		{Index: 1, Status: model.StatusAccepted},
		{Index: 2, Status: model.StatusSystemError, Message: "评测数据缺失"},
		{Index: 3, Status: model.StatusSystemError, Message: "另一个问题"},
	}
	if got := systemMessage(results); got != "评测数据缺失" {
		t.Errorf("systemMessage = %q", got)
	}
	if got := systemMessage(nil); got != "" {
		t.Errorf("空结果的 systemMessage = %q", got)
	}
}
