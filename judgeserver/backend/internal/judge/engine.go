package judge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"akioi/internal/config"
	"akioi/internal/logger"
	"akioi/internal/model"
)

var (
	userTimeRe = regexp.MustCompile(`User time \(seconds\):\s*([0-9.]+)`)
	maxRSSRe   = regexp.MustCompile(`Maximum resident set size \(kbytes\):\s*([0-9]+)`)
)

// Engine 是评测主流程：编译一次，逐测试点独立执行与比对。
// 数据全部来自主站下发的任务，节点本地不再访问业务库。
type Engine struct {
	cfg *config.Config
	sb  *Sandbox
}

func New(cfg *config.Config) *Engine {
	return &Engine{cfg: cfg, sb: NewSandbox(&cfg.Judge)}
}

// Judge 执行一次评测，返回回传给主站的结果。
func (e *Engine) Judge(ctx context.Context, task *model.JudgeTask, onCase func(done, total int)) *model.JudgeResult {
	res := &model.JudgeResult{
		SubmissionID: task.SubmissionID,
		JudgeToken:   task.JudgeToken,
		Status:       model.StatusSystemError,
		Results:      []model.TestcaseResult{},
	}

	lang, err := ResolveLanguage(task.Language, &e.cfg.Judge)
	if err != nil {
		res.JudgeMessage = err.Error()
		return res
	}
	if len(task.Testcases) == 0 {
		res.JudgeMessage = "该题目没有测试数据"
		return res
	}

	workName := "submission_" + strconv.FormatUint(uint64(task.SubmissionID), 10)
	// 本地目录用于评测进程自身读写，宿主路径用于 docker 挂载到沙箱，两者在容器内运行时不同。
	localWorkDir := filepath.Join(e.cfg.Judge.WorkDir, workName)
	hostWorkDir := filepath.Join(e.hostWorkDir(), workName)
	if err := os.MkdirAll(localWorkDir, 0o777); err != nil {
		res.JudgeMessage = "无法创建评测工作目录"
		return res
	}
	defer func() { _ = os.RemoveAll(localWorkDir) }()

	if err := os.WriteFile(filepath.Join(localWorkDir, lang.SourceName), []byte(task.Code), 0o666); err != nil {
		res.JudgeMessage = "无法写入源码文件"
		return res
	}

	compileCtx, cancelCompile := context.WithTimeout(ctx, time.Duration(e.cfg.Judge.CompileTimeoutSec)*time.Second)
	defer cancelCompile()
	compileRes, err := e.sb.Run(compileCtx, RunSpec{
		Name:         e.containerName(task.SubmissionID, "compile"),
		ShellCommand: strings.Join(lang.CompileArgs, " "),
		MemoryMB:     e.cfg.Judge.CompileMemoryMB,
		CPUs:         "1",
		Timeout:      time.Duration(e.cfg.Judge.CompileTimeoutSec) * time.Second,
		WorkHostDir:  hostWorkDir,
	})
	if err != nil {
		res.JudgeMessage = "编译沙箱启动失败: " + err.Error()
		return res
	}
	if compileRes.ExitCode != 0 {
		msg := compileRes.Stderr
		if strings.TrimSpace(msg) == "" {
			msg = "编译失败，无编译器输出"
		}
		logger.Judgef("提交 %d 编译失败 exit=%d", task.SubmissionID, compileRes.ExitCode)
		res.Status = model.StatusCompileError
		res.CompileError = msg
		return res
	}

	spjPath := ""
	if task.CompareMode == model.CompareModeSPJ {
		spjPath, err = e.prepareSPJ(ctx, task, localWorkDir, hostWorkDir)
		if err != nil {
			res.JudgeMessage = err.Error()
			return res
		}
	}

	problemDir := strconv.FormatUint(uint64(task.ProblemID), 10)
	paths := casePaths{
		spjPath:       spjPath,
		localWork:     localWorkDir,
		hostWork:      hostWorkDir,
		localTestdata: filepath.Join(e.cfg.Judge.TestdataDir, problemDir),
		hostTestdata:  filepath.Join(e.hostTestdataDir(), problemDir),
		outputLimit:   int64(e.cfg.Judge.OutputLimitMB) * 1024 * 1024,
	}

	finalStatus := model.StatusAccepted
	groupFailed := make(map[int]bool)
	groupSeen := make(map[int]int)
	totalScore := 0
	maxTimeMS := 0
	maxMemoryKB := 0

	for _, tc := range task.Testcases {
		item, fatal := e.runCase(ctx, task, lang, tc, paths)
		res.Results = append(res.Results, item)
		if tc.Group > 0 {
			groupSeen[tc.Group]++
			if item.Status != model.StatusAccepted {
				groupFailed[tc.Group] = true
			}
		} else {
			totalScore += item.Score
		}
		if onCase != nil {
			onCase(len(res.Results), len(task.Testcases))
		}
		if item.TimeMS > maxTimeMS {
			maxTimeMS = item.TimeMS
		}
		if item.MemoryKB > maxMemoryKB {
			maxMemoryKB = item.MemoryKB
		}
		if finalStatus == model.StatusAccepted && item.Status != model.StatusAccepted {
			finalStatus = item.Status
		}
		if !fatal {
			continue
		}
		// 测试数据缺失这类环境问题对每个测试点都一样，没必要继续跑完剩下的。
		finalStatus = model.StatusSystemError
		logger.Judgef("提交 %d 在测试点 #%d 遇到评测环境问题，提前结束", task.SubmissionID, item.Index)
		break
	}

	// 子任务按「组内全部通过才给分」结算，独立测试点已经在上面的循环里累加。
	for _, st := range task.Subtasks {
		if groupFailed[st.ID] || groupSeen[st.ID] < len(st.Cases) {
			continue
		}
		totalScore += st.Score
	}

	// 评测环境故障时不计分，也不让环境问题看起来像选手的运行时错误。
	if finalStatus == model.StatusSystemError {
		totalScore = 0
		res.JudgeMessage = systemMessage(res.Results)
	}

	res.Status = finalStatus
	res.Score = totalScore
	res.TimeMS = maxTimeMS
	res.MemoryKB = maxMemoryKB
	logger.Judgef("提交 %d 评测完成 status=%s score=%d time=%dms memory=%dKB",
		task.SubmissionID, finalStatus, totalScore, maxTimeMS, maxMemoryKB)
	return res
}

// casePaths 汇总单个测试点评测需要用到的目录与限制。
type casePaths struct {
	spjPath       string
	localWork     string
	hostWork      string
	localTestdata string
	hostTestdata  string
	outputLimit   int64
}

// runCase 执行单个测试点。第二个返回值表示评测环境本身出了问题，整轮评测可以提前结束。
func (e *Engine) runCase(ctx context.Context, task *model.JudgeTask, lang Language,
	tc model.TaskTestcase, paths casePaths) (model.TestcaseResult, bool) {

	item := model.TestcaseResult{Index: tc.Index, Group: tc.Group, Status: model.StatusSystemError}
	timeLimit := time.Duration(task.TimeLimitMS) * time.Millisecond
	if timeLimit <= 0 {
		timeLimit = time.Second
	}
	caseTimeout := timeLimit*2 + 3*time.Second
	outName := fmt.Sprintf("case%d.out", tc.Index)
	errName := fmt.Sprintf("case%d.err", tc.Index)
	timeName := fmt.Sprintf("case%d.time", tc.Index)
	timePath := filepath.Join(paths.localWork, timeName)
	errPath := filepath.Join(paths.localWork, errName)

	runCtx, cancel := context.WithTimeout(ctx, caseTimeout)
	defer cancel()
	res, err := e.sb.Run(runCtx, RunSpec{
		Name:             e.containerName(task.SubmissionID, "case"+strconv.Itoa(tc.Index)),
		ShellCommand:     runShell(lang.RunPath, tc.Index, timeName, outName, errName),
		MemoryMB:         task.MemoryLimitMB,
		CPUs:             "1",
		Timeout:          caseTimeout,
		WorkHostDir:      paths.hostWork,
		TestdataHostDir:  paths.hostTestdata,
		OutputLimitBytes: paths.outputLimit * 2,
	})
	if err != nil {
		item.Message = "沙箱执行失败: " + err.Error()
		return item, false
	}

	userMS, maxRSSKB := parseTimeReport(timePath)
	item.TimeMS = userMS
	item.MemoryKB = maxRSSKB
	if item.TimeMS == 0 {
		item.TimeMS = res.WallMS
	}
	// /usr/bin/time 只有真正启动选手程序后才会写报告；没有报告说明命令根本没跑起来。
	reportWritten := fileExists(timePath)

	if res.TimedOut || (userMS > 0 && userMS > task.TimeLimitMS) {
		item.Status = model.StatusTimeLimitExceeded
		item.Message = fmt.Sprintf("超出时间限制 %dms", task.TimeLimitMS)
		return item, false
	}
	if res.ExitCode != 0 {
		if res.ExitCode == 137 || (maxRSSKB > 0 && maxRSSKB > task.MemoryLimitMB*1024) {
			item.Status = model.StatusMemoryLimitExceeded
			item.Message = fmt.Sprintf("超出内存限制 %dMB", task.MemoryLimitMB)
			return item, false
		}
		item.ExitCode = res.ExitCode
		item.Stderr = mergeStderr(errPath, res.Stderr)
		if !reportWritten {
			// 选手程序没有真正跑起来：多半是测试数据没挂进沙箱，按环境故障上报。
			item.Status = model.StatusSystemError
			item.Message = environmentMessage(res.ExitCode, item.Stderr)
			return item, true
		}
		item.Status = model.StatusRuntimeError
		item.Message = runtimeMessage(res.ExitCode)
		return item, false
	}

	outInfo, err := os.Stat(filepath.Join(paths.localWork, outName))
	if err == nil && outInfo.Size() > paths.outputLimit {
		item.Status = model.StatusOutputLimitExceeded
		item.Message = fmt.Sprintf("输出超过 %dMB", e.cfg.Judge.OutputLimitMB)
		return item, false
	}

	actual, err := os.ReadFile(filepath.Join(paths.localWork, outName))
	if err != nil {
		item.Status = model.StatusSystemError
		item.Message = "无法读取选手输出"
		return item, false
	}
	expected, err := os.ReadFile(filepath.Join(paths.localTestdata, strconv.Itoa(tc.Index)+".out"))
	if err != nil {
		item.Status = model.StatusSystemError
		item.Message = "无法读取标准输出，测试数据可能不完整"
		return item, true
	}

	if task.CompareMode == model.CompareModeSPJ {
		return e.runSPJ(ctx, task, tc, paths)
	}

	cmp := Compare(task.CompareMode, task.FloatEps, expected, actual)
	if !cmp.Accepted {
		item.Status = model.StatusWrongAnswer
		item.Message = cmp.Message
		return item, false
	}
	item.Status = model.StatusAccepted
	item.Score = tc.Score
	return item, false
}

func (e *Engine) hostTestdataDir() string {
	if e.cfg.Judge.HostTestdataDir != "" {
		return e.cfg.Judge.HostTestdataDir
	}
	return e.cfg.Judge.TestdataDir
}

// hostWorkDir 返回评测工作目录在宿主机上的路径，供 docker 挂载使用。
func (e *Engine) hostWorkDir() string {
	if e.cfg.Judge.HostWorkDir != "" {
		return e.cfg.Judge.HostWorkDir
	}
	return e.cfg.Judge.WorkDir
}

func (e *Engine) containerName(submissionID uint, phase string) string {
	return fmt.Sprintf("akioi-judge-%d-%s", submissionID, phase)
}

func parseTimeReport(path string) (int, int) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0
	}
	text := string(b)
	userMS := 0
	memKB := 0
	if m := userTimeRe.FindStringSubmatch(text); len(m) == 2 {
		if sec, err := strconv.ParseFloat(m[1], 64); err == nil {
			userMS = int(sec*1000 + 0.5)
		}
	}
	if m := maxRSSRe.FindStringSubmatch(text); len(m) == 2 {
		if kb, err := strconv.Atoi(m[1]); err == nil {
			memKB = kb
		}
	}
	return userMS, memKB
}

// prepareSPJ 把主站下发的 Special Judge 源码写进工作目录并编译成 /work/spj。
func (e *Engine) prepareSPJ(ctx context.Context, task *model.JudgeTask, localWorkDir, hostWorkDir string) (string, error) {
	if strings.TrimSpace(task.SPJCode) == "" {
		return "", fmt.Errorf("题目开启了 SPJ，但没有下发 SPJ 程序")
	}
	lang := task.SPJLanguage
	if lang == "" {
		lang = model.LangCpp
	}
	compiler := e.cfg.Judge.CppCompiler
	std := e.cfg.Judge.CppStd
	flags := e.cfg.Judge.CppFlags
	ext := "cpp"
	switch lang {
	case model.LangCpp:
	case model.LangC:
		compiler = e.cfg.Judge.CCompiler
		std = e.cfg.Judge.CStd
		flags = e.cfg.Judge.CFlags
		ext = "c"
	default:
		return "", fmt.Errorf("SPJ 语言不支持: %s", lang)
	}
	source := filepath.Join(localWorkDir, "spj."+ext)
	if err := os.WriteFile(source, []byte(task.SPJCode), 0o666); err != nil {
		return "", fmt.Errorf("无法写入 SPJ 源文件")
	}
	parts := []string{compiler}
	if std != "" {
		parts = append(parts, "-std="+std)
	}
	parts = append(parts, splitFlags(flags)...)
	parts = append(parts, "-o", "/work/spj", "/work/spj."+ext)
	timeout := time.Duration(e.cfg.Judge.CompileTimeoutSec) * time.Second
	compileCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := e.sb.Run(compileCtx, RunSpec{
		Name:         e.containerName(task.SubmissionID, "spj"),
		ShellCommand: strings.Join(parts, " "),
		MemoryMB:     e.cfg.Judge.CompileMemoryMB,
		CPUs:         "1",
		Timeout:      timeout,
		WorkHostDir:  hostWorkDir,
	})
	if err != nil {
		return "", fmt.Errorf("SPJ 编译沙箱启动失败: %v", err)
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = "编译失败，无编译器输出"
		}
		return "", fmt.Errorf("SPJ 编译失败: %s", truncate(msg, 2000))
	}
	return "/work/spj", nil
}

// runSPJ 用题目自带的 SPJ 程序判定单个测试点：退出码 0 通过，非 0 判错，stderr 作为提示。
func (e *Engine) runSPJ(ctx context.Context, task *model.JudgeTask, tc model.TaskTestcase,
	paths casePaths) (model.TestcaseResult, bool) {

	item := model.TestcaseResult{Index: tc.Index, Group: tc.Group, Status: model.StatusSystemError}
	if paths.spjPath == "" {
		item.Message = "题目没有可用的 SPJ 程序"
		return item, true
	}
	command := fmt.Sprintf("%s /box/%d.in /box/%d.out /work/case%d.out",
		paths.spjPath, tc.Index, tc.Index, tc.Index)
	timeout := 15 * time.Second
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := e.sb.Run(runCtx, RunSpec{
		Name:             e.containerName(task.SubmissionID, "spj"+strconv.Itoa(tc.Index)),
		ShellCommand:     command,
		MemoryMB:         256,
		CPUs:             "1",
		Timeout:          timeout,
		WorkHostDir:      paths.hostWork,
		TestdataHostDir:  paths.hostTestdata,
		OutputLimitBytes: 1 << 20,
	})
	if err != nil {
		item.Message = "SPJ 执行失败: " + err.Error()
		return item, false
	}
	if res.TimedOut {
		item.Message = "SPJ 运行超时"
		return item, true
	}
	if res.ExitCode == 0 {
		item.Status = model.StatusAccepted
		item.Score = tc.Score
		return item, false
	}
	message := strings.TrimSpace(res.Stderr)
	if message == "" {
		message = "SPJ 判定为不通过"
	}
	item.Status = model.StatusWrongAnswer
	item.Message = truncate(message, 200)
	return item, false
}
