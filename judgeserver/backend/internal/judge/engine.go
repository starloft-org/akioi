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
func (e *Engine) Judge(ctx context.Context, task *model.JudgeTask) *model.JudgeResult {
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

	problemDir := strconv.FormatUint(uint64(task.ProblemID), 10)
	paths := casePaths{
		localWork:     localWorkDir,
		hostWork:      hostWorkDir,
		localTestdata: filepath.Join(e.cfg.Judge.TestdataDir, problemDir),
		hostTestdata:  filepath.Join(e.hostTestdataDir(), problemDir),
		outputLimit:   int64(e.cfg.Judge.OutputLimitMB) * 1024 * 1024,
	}

	finalStatus := model.StatusAccepted
	totalScore := 0
	maxTimeMS := 0
	maxMemoryKB := 0

	for _, tc := range task.Testcases {
		item := e.runCase(ctx, task, lang, tc, paths)
		res.Results = append(res.Results, item)
		totalScore += item.Score
		if item.TimeMS > maxTimeMS {
			maxTimeMS = item.TimeMS
		}
		if item.MemoryKB > maxMemoryKB {
			maxMemoryKB = item.MemoryKB
		}
		if finalStatus == model.StatusAccepted && item.Status != model.StatusAccepted {
			finalStatus = item.Status
		}
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
	localWork     string
	hostWork      string
	localTestdata string
	hostTestdata  string
	outputLimit   int64
}

// runCase 执行单个测试点并得出状态、耗时与内存。
func (e *Engine) runCase(ctx context.Context, task *model.JudgeTask, lang Language,
	tc model.TaskTestcase, paths casePaths) model.TestcaseResult {

	item := model.TestcaseResult{Index: tc.Index, Status: model.StatusSystemError}
	timeLimit := time.Duration(task.TimeLimitMS) * time.Millisecond
	if timeLimit <= 0 {
		timeLimit = time.Second
	}
	caseTimeout := timeLimit*2 + 3*time.Second
	outName := fmt.Sprintf("case%d.out", tc.Index)
	errName := fmt.Sprintf("case%d.err", tc.Index)
	timeName := fmt.Sprintf("case%d.time", tc.Index)

	shell := fmt.Sprintf("/usr/bin/time -v -o /work/%s %s < /box/%d.in > /work/%s 2> /work/%s",
		timeName, lang.RunPath, tc.Index, outName, errName)

	runCtx, cancel := context.WithTimeout(ctx, caseTimeout)
	defer cancel()
	res, err := e.sb.Run(runCtx, RunSpec{
		Name:             e.containerName(task.SubmissionID, "case"+strconv.Itoa(tc.Index)),
		ShellCommand:     shell,
		MemoryMB:         task.MemoryLimitMB,
		CPUs:             "1",
		Timeout:          caseTimeout,
		WorkHostDir:      paths.hostWork,
		TestdataHostDir:  paths.hostTestdata,
		OutputLimitBytes: paths.outputLimit * 2,
	})
	if err != nil {
		item.Message = "沙箱执行失败: " + err.Error()
		return item
	}

	userMS, maxRSSKB := parseTimeReport(filepath.Join(paths.localWork, timeName))
	item.TimeMS = userMS
	item.MemoryKB = maxRSSKB
	if item.TimeMS == 0 {
		item.TimeMS = res.WallMS
	}

	if res.TimedOut || (userMS > 0 && userMS > task.TimeLimitMS) {
		item.Status = model.StatusTimeLimitExceeded
		item.Message = fmt.Sprintf("超出时间限制 %dms", task.TimeLimitMS)
		return item
	}
	if res.ExitCode != 0 {
		if res.ExitCode == 137 || maxRSSKB > task.MemoryLimitMB*1024 {
			item.Status = model.StatusMemoryLimitExceeded
			item.Message = fmt.Sprintf("超出内存限制 %dMB", task.MemoryLimitMB)
			return item
		}
		item.Status = model.StatusRuntimeError
		item.Message = runtimeMessage(res.ExitCode, filepath.Join(paths.localWork, errName))
		return item
	}

	outInfo, err := os.Stat(filepath.Join(paths.localWork, outName))
	if err == nil && outInfo.Size() > paths.outputLimit {
		item.Status = model.StatusOutputLimitExceeded
		item.Message = fmt.Sprintf("输出超过 %dMB", e.cfg.Judge.OutputLimitMB)
		return item
	}

	actual, err := os.ReadFile(filepath.Join(paths.localWork, outName))
	if err != nil {
		item.Status = model.StatusSystemError
		item.Message = "无法读取选手输出"
		return item
	}
	expected, err := os.ReadFile(filepath.Join(paths.localTestdata, strconv.Itoa(tc.Index)+".out"))
	if err != nil {
		item.Status = model.StatusSystemError
		item.Message = "无法读取标准输出"
		return item
	}

	cmp := Compare(task.CompareMode, task.FloatEps, expected, actual)
	if !cmp.Accepted {
		item.Status = model.StatusWrongAnswer
		item.Message = cmp.Message
		return item
	}
	item.Status = model.StatusAccepted
	item.Score = tc.Score
	return item
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

func runtimeMessage(exitCode int, stderrPath string) string {
	msg := fmt.Sprintf("运行时错误，退出码 %d", exitCode)
	if b, err := os.ReadFile(stderrPath); err == nil && len(b) > 0 {
		msg += "：" + truncate(strings.TrimSpace(string(b)), 200)
	}
	return msg
}
