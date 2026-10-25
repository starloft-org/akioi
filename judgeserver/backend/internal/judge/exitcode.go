package judge

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"akioi/internal/model"
)

// 沙箱内前置检查失败时使用的退出码。这些退出码只会在
// /usr/bin/time 尚未启动选手程序时出现，借此把环境故障和选手的运行时错误区分开。
const (
	exitCodeTestdataMissing = 97
	exitCodeProgramMissing  = 98
	exitCodeTimeMissing     = 99
)

// stderrLimit 限制单个测试点保存的错误输出长度，避免结果 JSON 过大。
const stderrLimit = 2000

var (
	timeStatusRe = regexp.MustCompile(`(?m)^\s*Command (?:exited with non-zero status|terminated by signal) \d+\s*$`)
	blankLineRe  = regexp.MustCompile(`\n{3,}`)
)

// signalDescriptions 把常见信号翻译成选手能看懂的原因。
var signalDescriptions = map[int]string{
	1:  "收到挂起信号 SIGHUP",
	2:  "收到中断信号 SIGINT",
	3:  "收到退出信号 SIGQUIT",
	4:  "非法指令 SIGILL，可能是编译产物损坏或未定义行为",
	5:  "调试陷阱 SIGTRAP",
	6:  "程序被中止 SIGABRT，常见于断言失败或未捕获的异常",
	7:  "总线错误 SIGBUS，常见于内存对齐或内存映射异常",
	8:  "算术错误 SIGFPE，常见于除以零或整数溢出",
	9:  "进程被强制终止 SIGKILL，常见于内存超限",
	11: "段错误 SIGSEGV，程序访问了非法内存（数组越界、空指针解引用等）",
	13: "管道错误 SIGPIPE，向已关闭的管道写数据",
	14: "收到定时器信号 SIGALRM",
	15: "进程被终止 SIGTERM",
	24: "超出 CPU 时间限制 SIGXCPU",
	25: "超出文件大小限制 SIGXFSZ，输出可能过大",
}

// exitDescription 把退出码翻译成可读原因；大于 128 的退出码按信号终止解释。
func exitDescription(code int) string {
	if code > 128 {
		sig := code - 128
		if desc, ok := signalDescriptions[sig]; ok {
			return fmt.Sprintf("%s（退出码 %d）", desc, code)
		}
		return fmt.Sprintf("进程被信号 %d 终止（退出码 %d）", sig, code)
	}
	if code == 0 {
		return "正常结束"
	}
	return fmt.Sprintf("进程以退出码 %d 结束", code)
}

// runtimeMessage 生成运行时错误的简要说明，完整输出放在 Stderr 字段。
func runtimeMessage(code int) string {
	if code > 128 {
		return "运行时错误：" + exitDescription(code)
	}
	return fmt.Sprintf("运行时错误，退出码 %d", code)
}

// environmentMessage 说明评测环境自身的问题，避免把环境故障显示成选手的运行时错误。
// 前置检查已给出具体原因的退出码不再拼接 stderr，其余情况补充 stderr 首行。
func environmentMessage(code int, stderr string) string {
	msg := environmentBaseMessage(code)
	switch code {
	case exitCodeTestdataMissing, exitCodeProgramMissing, exitCodeTimeMissing:
		return msg
	}
	if line := firstLine(stderr); line != "" {
		msg += "：" + truncateRunes(line, 120)
	}
	return msg
}

func environmentBaseMessage(code int) string {
	switch code {
	case exitCodeTestdataMissing:
		return "评测数据缺失：沙箱内找不到该测试点的输入文件，请检查评测节点的测试数据挂载"
	case exitCodeProgramMissing:
		return "评测程序缺失：编译产物在沙箱内不存在或不可执行"
	case exitCodeTimeMissing:
		return "评测环境异常：沙箱内缺少 /usr/bin/time，无法采集耗时与内存"
	case 126, 127:
		return fmt.Sprintf("评测环境异常：无法执行选手程序（退出码 %d），请检查编译产物与沙箱镜像", code)
	}
	return fmt.Sprintf("评测环境异常：评测程序未能启动（退出码 %d）", code)
}

// firstLine 取第一行非空内容，用于把 stderr 的关键信息并进简要说明。
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}

// runShell 拼出单个测试点的执行脚本。先检查输入文件、可执行文件与 /usr/bin/time，
// 缺失时直接给出明确原因；否则 sh 会因重定向失败以退出码 2 结束，且看不到任何错误输出。
func runShell(runPath string, index int, timeName, outName, errName string) string {
	inPath := "/box/" + strconv.Itoa(index) + ".in"
	script := `[ -r {in} ] || { echo "测试数据缺失：沙箱内找不到 {in}" >&2; exit {codeTestdata}; }
[ -x {run} ] || { echo "评测程序缺失：{run} 不存在或不可执行" >&2; exit {codeProgram}; }
[ -x /usr/bin/time ] || { echo "评测环境缺少 /usr/bin/time，无法采集耗时与内存" >&2; exit {codeTime}; }
/usr/bin/time -v -o /work/{time} {run} < {in} > /work/{out} 2> /work/{err}`
	return strings.NewReplacer(
		"{in}", inPath,
		"{run}", runPath,
		"{time}", timeName,
		"{out}", outName,
		"{err}", errName,
		"{codeTestdata}", strconv.Itoa(exitCodeTestdataMissing),
		"{codeProgram}", strconv.Itoa(exitCodeProgramMissing),
		"{codeTime}", strconv.Itoa(exitCodeTimeMissing),
	).Replace(script)
}

// cleanStderr 去掉 /usr/bin/time 自己打印的状态行与多余空行。
func cleanStderr(raw string) string {
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	s = timeStatusRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	s = blankLineRe.ReplaceAllString(s, "\n\n")
	return truncateRunes(s, stderrLimit)
}

// mergeStderr 合并选手程序的错误输出与沙箱自身的错误输出。
func mergeStderr(errPath, sandboxStderr string) string {
	parts := make([]string, 0, 2)
	// 按字符上限的四倍读取，避免截断时把多字节字符切坏。
	if text := cleanStderr(readHead(errPath, stderrLimit*4)); text != "" {
		parts = append(parts, text)
	}
	if text := cleanStderr(sandboxStderr); text != "" && !strings.Contains(strings.Join(parts, "\n"), text) {
		parts = append(parts, text)
	}
	return truncateRunes(strings.Join(parts, "\n"), stderrLimit)
}

// readHead 只读取文件开头的一段，避免被超大输出拖垮评测进程。
func readHead(path string, max int64) string {
	f, err := os.Open(path)
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

// fileExists 判断文件是否已写入内容；/usr/bin/time 的报告为空说明它没有真正运行选手程序。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// truncateRunes 按字符截断，避免中文被截成半个字。
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// systemMessage 取第一条系统错误作为整份提交的说明，展示在评测详情顶部。
func systemMessage(results []model.TestcaseResult) string {
	for _, item := range results {
		if item.Status == model.StatusSystemError && item.Message != "" {
			return truncateRunes(item.Message, 200)
		}
	}
	return ""
}
