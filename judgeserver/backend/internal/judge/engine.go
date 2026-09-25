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
	"akioi/internal/redis"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	userTimeRe = regexp.MustCompile(`User time \(seconds\):\s*([0-9.]+)`)
	maxRSSRe   = regexp.MustCompile(`Maximum resident set size \(kbytes\):\s*([0-9]+)`)
)

// Engine 是评测主流程：编译一次，逐测试点独立执行与比对。
type Engine struct {
	cfg *config.Config
	db  *gorm.DB
	rdb *goredis.Client
	sb  *Sandbox
}

func New(cfg *config.Config, db *gorm.DB, rdb *goredis.Client) *Engine {
	return &Engine{cfg: cfg, db: db, rdb: rdb, sb: NewSandbox(&cfg.Judge)}
}

// Judge 执行一次评测；仅当 submission 的 judge_token 与 token 一致时回写结果。
func (e *Engine) Judge(ctx context.Context, submissionID uint, token string) error {
	release, owned, err := e.acquire(ctx, submissionID, token)
	if err != nil {
		// 锁不可用时继续评测：结果以 judge_token 校验，重复写回的值相同。
		logger.Errorf("获取评测占用锁失败 submission=%d err=%v", submissionID, err)
	} else if !owned {
		logger.Judgef("提交 %d 正由其它评测节点处理，跳过本次任务", submissionID)
		return nil
	} else {
		defer release()
	}

	var sub model.Submission
	if err := e.db.First(&sub, submissionID).Error; err != nil {
		return fmt.Errorf("提交 %d 不存在: %w", submissionID, err)
	}
	if sub.JudgeToken != token {
		// 已被重新投递的任务，放弃本次结果。
		return nil
	}

	if err := e.db.Model(&model.Submission{}).Where("id = ?", sub.ID).
		Updates(map[string]any{"status": model.StatusJudging}).Error; err != nil {
		return err
	}

	var prob model.Problem
	if err := e.db.First(&prob, sub.ProblemID).Error; err != nil {
		return e.finish(sub, token, model.StatusSystemError, 0, nil, "", "题目不存在")
	}

	lang, err := ResolveLanguage(sub.Language, &e.cfg.Judge)
	if err != nil {
		return e.finish(sub, token, model.StatusSystemError, 0, nil, "", err.Error())
	}

	var cases []model.Testcase
	if err := e.db.Where("problem_id = ?", prob.ID).Order("case_index asc").Find(&cases).Error; err != nil {
		return err
	}
	if len(cases) == 0 {
		return e.finish(sub, token, model.StatusSystemError, 0, nil, "", "该题目没有测试数据")
	}

	workName := "submission_" + strconv.FormatUint(uint64(sub.ID), 10)
	// 本地目录用于评测进程自身读写，宿主路径用于 docker 挂载到沙箱，两者在容器内运行时不同。
	localWorkDir := filepath.Join(e.cfg.Judge.WorkDir, workName)
	hostWorkDir := filepath.Join(e.hostWorkDir(), workName)
	if err := os.MkdirAll(localWorkDir, 0o777); err != nil {
		return e.finish(sub, token, model.StatusSystemError, 0, nil, "", "无法创建评测工作目录")
	}
	defer func() { _ = os.RemoveAll(localWorkDir) }()

	if err := os.WriteFile(filepath.Join(localWorkDir, lang.SourceName), []byte(sub.Code), 0o666); err != nil {
		return e.finish(sub, token, model.StatusSystemError, 0, nil, "", "无法写入源码文件")
	}

	compileCtx, cancelCompile := context.WithTimeout(ctx, time.Duration(e.cfg.Judge.CompileTimeoutSec)*time.Second)
	defer cancelCompile()
	compileRes, err := e.sb.Run(compileCtx, RunSpec{
		Name:         e.containerName(sub.ID, "compile"),
		ShellCommand: strings.Join(lang.CompileArgs, " "),
		MemoryMB:     e.cfg.Judge.CompileMemoryMB,
		CPUs:         "1",
		Timeout:      time.Duration(e.cfg.Judge.CompileTimeoutSec) * time.Second,
		WorkHostDir:  hostWorkDir,
	})
	if err != nil {
		return e.finish(sub, token, model.StatusSystemError, 0, nil, "", "编译沙箱启动失败: "+err.Error())
	}
	if compileRes.ExitCode != 0 {
		msg := compileRes.Stderr
		if strings.TrimSpace(msg) == "" {
			msg = "编译失败，无编译器输出"
		}
		logger.Judgef("提交 %d 编译失败 exit=%d", sub.ID, compileRes.ExitCode)
		return e.finish(sub, token, model.StatusCompileError, 0, nil, msg, "")
	}

	problemDir := strconv.FormatUint(uint64(prob.ID), 10)
	localTestdata := filepath.Join(e.cfg.Judge.TestdataDir, problemDir)
	hostTestdata := filepath.Join(e.hostTestdataDir(), problemDir)
	outputLimit := int64(e.cfg.Judge.OutputLimitMB) * 1024 * 1024

	results := make([]model.TestcaseResult, 0, len(cases))
	finalStatus := model.StatusAccepted
	totalScore := 0
	maxTimeMS := 0
	maxMemoryKB := 0

	for _, tc := range cases {
		item := e.runCase(ctx, &sub, &prob, lang, tc, casePaths{
			localWork:     localWorkDir,
			hostWork:      hostWorkDir,
			localTestdata: localTestdata,
			hostTestdata:  hostTestdata,
			outputLimit:   outputLimit,
		})
		results = append(results, item)
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

	if err := e.finish(sub, token, finalStatus, totalScore, results, "", ""); err != nil {
		return err
	}
	if err := e.db.Model(&model.Submission{}).Where("id = ? AND judge_token = ?", sub.ID, token).
		Updates(map[string]any{"time_ms": maxTimeMS, "memory_kb": maxMemoryKB}).Error; err != nil {
		return err
	}
	if finalStatus == model.StatusAccepted {
		e.markAccepted(prob.ID)
	}
	logger.Judgef("提交 %d 评测完成 status=%s score=%d time=%dms memory=%dKB", sub.ID, finalStatus, totalScore, maxTimeMS, maxMemoryKB)
	return nil
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
func (e *Engine) runCase(ctx context.Context, sub *model.Submission, prob *model.Problem, lang Language,
	tc model.Testcase, paths casePaths) model.TestcaseResult {

	item := model.TestcaseResult{Index: tc.Index, Status: model.StatusSystemError}
	timeLimit := time.Duration(prob.TimeLimitMS) * time.Millisecond
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
		Name:             e.containerName(sub.ID, "case"+strconv.Itoa(tc.Index)),
		ShellCommand:     shell,
		MemoryMB:         prob.MemoryLimitMB,
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

	if res.TimedOut || (userMS > 0 && userMS > prob.TimeLimitMS) {
		item.Status = model.StatusTimeLimitExceeded
		item.Message = fmt.Sprintf("超出时间限制 %dms", prob.TimeLimitMS)
		return item
	}
	if res.ExitCode != 0 {
		if res.ExitCode == 137 || maxRSSKB > prob.MemoryLimitMB*1024 {
			item.Status = model.StatusMemoryLimitExceeded
			item.Message = fmt.Sprintf("超出内存限制 %dMB", prob.MemoryLimitMB)
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

	cmp := Compare(prob.CompareMode, prob.FloatEps, expected, actual)
	if !cmp.Accepted {
		item.Status = model.StatusWrongAnswer
		item.Message = cmp.Message
		return item
	}
	item.Status = model.StatusAccepted
	item.Score = tc.Score
	return item
}

// finish 回写终态结果，仅当 judge_token 未被更新时生效。
func (e *Engine) finish(sub model.Submission, token, status string, score int,
	results []model.TestcaseResult, compileError, judgeMessage string) error {

	now := time.Now()
	updates := map[string]any{
		"status":        status,
		"score":         score,
		"judged_at":     &now,
		"judge_message": judgeMessage,
		"compile_error": compileError,
	}
	if results != nil {
		sub.SetResults(results)
		updates["testcase_results"] = sub.ResultsJSON
	}
	if compileError != "" {
		updates["compile_error"] = truncate(compileError, 8000)
	}
	res := e.db.Model(&model.Submission{}).Where("id = ? AND judge_token = ?", sub.ID, token).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

// acquire 抢占本次评测任务的占用锁，返回释放函数与是否抢到锁。
// 多节点同时收到同一 token 的任务时只有抢到锁的节点执行评测。
func (e *Engine) acquire(ctx context.Context, submissionID uint, token string) (func(), bool, error) {
	key := fmt.Sprintf("%s%d:%s", redis.KeyJudgeLockPrefix, submissionID, token)
	ttl := time.Duration(e.cfg.Judge.TaskTimeoutSec+60) * time.Second
	ok, err := e.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	release := func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = e.rdb.Eval(releaseCtx, releaseLockScript, []string{key}, token).Err()
	}
	return release, true, nil
}

// releaseLockScript 仅在锁值仍属于自己时删除，避免误删超时后被他节点抢占的锁。
const releaseLockScript = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) end return 0`

// markAccepted 重算题目的通过人数，口径为去重后的通过用户数；
// 单条 SQL 完成，重测与多节点并发都不会重复计数。
func (e *Engine) markAccepted(problemID uint) {
	err := e.db.Exec(`UPDATE problems SET accepted_count = (
	SELECT COUNT(DISTINCT s.user_id) FROM submissions s
	WHERE s.problem_id = problems.id AND s.status = ?) WHERE id = ?`,
		model.StatusAccepted, problemID).Error
	if err != nil {
		logger.Errorf("重算题目通过人数失败 problem=%d err=%v", problemID, err)
	}
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
