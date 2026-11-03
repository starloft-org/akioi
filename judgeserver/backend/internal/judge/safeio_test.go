package judge

import (
	"os"
	"path/filepath"
	"testing"

	"akioi/internal/config"
	"akioi/internal/model"
)

// TestReadHeadRejectsSymlink 回归 S01：评测进程不能跟随选手创建的符号链接。
func TestReadHeadRejectsSymlink(t *testing.T) {
	tmp := t.TempDir()
	secret := filepath.Join(tmp, "AUDIT_CANARY")
	if err := os.WriteFile(secret, []byte("CANARY_SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(tmp, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(work, "case1.err")
	if err := os.Symlink(secret, link); err != nil {
		t.Skip("当前平台不支持创建符号链接:", err)
	}
	if got := readHeadSafe(link, 4096); got != "" {
		t.Fatalf("不应读取符号链接指向的内容，实际读到 %q", got)
	}
	if isRegularFile(link) {
		t.Fatal("符号链接不应被当作普通文件")
	}
}

// TestCompareFloatRejectsNaN 回归 S04：NaN 与无穷不能被判为答案一致。
func TestCompareFloatRejectsNaN(t *testing.T) {
	cases := []struct {
		name     string
		actual   string
		accepted bool
	}{
		{"NaN 替换标准答案", "NaN NaN", false},
		{"Inf 替换标准答案", "+Inf -Inf", false},
		{"正常数值", "123.0000001 456", true},
	}
	for _, c := range cases {
		got := Compare(model.CompareModeFloat, 1e-6, []byte("123 456"), []byte(c.actual))
		if got.Accepted != c.accepted {
			t.Fatalf("%s: accepted=%v, 期望 %v（消息 %q）", c.name, got.Accepted, c.accepted, got.Message)
		}
	}
	if got := Compare(model.CompareModeFloat, -1, []byte("1"), []byte("2")); got.Accepted {
		t.Fatal("负数误差不应当作有效误差")
	}
}

// TestStageCaseInputHidesAnswer 回归 S03：普通评测的沙箱目录只含输入，不含标准答案。
func TestStageCaseInputHidesAnswer(t *testing.T) {
	tmp := t.TempDir()
	testdata := filepath.Join(tmp, "testdata", "7")
	if err := os.MkdirAll(testdata, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testdata, "1.in"), []byte("input"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testdata, "1.out"), []byte("answer"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &Engine{cfg: &config.Config{Judge: config.JudgeConfig{WorkDir: filepath.Join(tmp, "work")}}}
	dir, cleanup, err := e.stageCaseInput(42, 1, testdata, false)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	localDir := filepath.Join(e.cfg.Judge.WorkDir, "sandbox_inputs", "42", "1")
	if _, err := os.Stat(filepath.Join(localDir, "1.in")); err != nil {
		t.Fatalf("输入文件应存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(localDir, "1.out")); err == nil {
		t.Fatalf("普通评测的沙箱目录不应包含标准答案（host=%s）", dir)
	}
	// SPJ 需要标准答案，单独确认它会复制答案。
	dir2, cleanup2, err := e.stageCaseInput(42, 1, testdata, true)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup2()
	if _, err := os.Stat(filepath.Join(e.cfg.Judge.WorkDir, "sandbox_inputs", "42", "1", "1.out")); err != nil {
		t.Fatalf("SPJ 目录应包含标准答案: %v（host=%s）", err, dir2)
	}
}
