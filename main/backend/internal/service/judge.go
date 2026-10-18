package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"akioi/internal/config"
	"akioi/internal/model"
	"akioi/internal/redis"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const maxCodeLength = 65536

var (
	ErrSubmitTooFast     = errors.New("提交过于频繁，请稍后再试")
	ErrProblemNotVisible = errors.New("题目当前不可提交")
	ErrContestNotRunning = errors.New("比赛未在进行中")
	ErrNotRegistered     = errors.New("请先报名该比赛")
	ErrCodeTooLong       = fmt.Errorf("代码长度超过 %d 字节", maxCodeLength)
	ErrLanguageInvalid   = errors.New("不支持的提交语言")
)

// JudgeService 负责提交的创建、校验与评测任务投递。
type JudgeService struct {
	db       *gorm.DB
	cfg      *config.Config
	rdb      *goredis.Client
	settings *SettingService
}

func NewJudge(db *gorm.DB, cfg *config.Config, rdb *goredis.Client, settings *SettingService) *JudgeService {
	return &JudgeService{db: db, cfg: cfg, rdb: rdb, settings: settings}
}

type SubmitInput struct {
	UserID    uint
	ProblemID uint
	ContestID uint
	Language  string
	Code      string
}

// Submit 校验提交条件后落库并投递评测任务。
func (s *JudgeService) Submit(ctx context.Context, in SubmitInput) (*model.Submission, error) {
	if in.Language != model.LangCpp && in.Language != model.LangC {
		return nil, ErrLanguageInvalid
	}
	if len(in.Code) == 0 {
		return nil, errors.New("代码不能为空")
	}
	if len(in.Code) > maxCodeLength {
		return nil, ErrCodeTooLong
	}

	var problem model.Problem
	if err := s.db.First(&problem, in.ProblemID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}

	now := time.Now()
	if in.ContestID > 0 {
		var contest model.Contest
		if err := s.db.First(&contest, in.ContestID).Error; err != nil {
			return nil, errors.New("比赛不存在")
		}
		if contest.Status(now) != model.ContestStatusRunning {
			return nil, ErrContestNotRunning
		}
		var cpCount int64
		if err := s.db.Model(&model.ContestProblem{}).
			Where("contest_id = ? AND problem_id = ?", contest.ID, problem.ID).Count(&cpCount).Error; err != nil {
			return nil, err
		}
		if cpCount == 0 {
			return nil, ErrProblemNotVisible
		}
		if contest.RegisterRequired {
			var joined int64
			if err := s.db.Model(&model.ContestParticipant{}).
				Where("contest_id = ? AND user_id = ?", contest.ID, in.UserID).Count(&joined).Error; err != nil {
				return nil, err
			}
			if joined == 0 {
				return nil, ErrNotRegistered
			}
		}
	} else if problem.Visibility != model.ProblemVisibilityPublic || problem.ContestOnly {
		return nil, ErrProblemNotVisible
	}

	submitInterval := s.settings.Current().SubmitIntervalSec
	if submitInterval > 0 {
		key := fmt.Sprintf("%s%d:%d", redis.KeySubmitLockPrefix, in.UserID, in.ProblemID)
		ok, err := s.rdb.SetNX(ctx, key, time.Now().Unix(), time.Duration(submitInterval)*time.Second).Result()
		if err != nil {
			return nil, fmt.Errorf("提交限频检查失败: %w", err)
		}
		if !ok {
			return nil, ErrSubmitTooFast
		}
	}

	token, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	sub := model.Submission{
		UserID:      in.UserID,
		ProblemID:   problem.ID,
		ContestID:   in.ContestID,
		Language:    in.Language,
		Code:        in.Code,
		CodeLength:  len(in.Code),
		Status:      model.StatusPending,
		JudgeToken:  token,
		ResultsJSON: "[]",
	}
	if err := s.db.Create(&sub).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.Problem{}).Where("id = ?", problem.ID).
		UpdateColumn("submit_count", gorm.Expr("submit_count + 1")).Error; err != nil {
		return nil, err
	}
	if err := s.dispatch(ctx, &sub, &problem); err != nil {
		// 没有节点接收任务时直接标记为系统错误，避免提交一直挂在等待评测。
		_ = s.db.Model(&model.Submission{}).Where("id = ?", sub.ID).Updates(map[string]any{
			"status":        model.StatusSystemError,
			"judge_message": "评测节点不可用: " + err.Error(),
			"judged_at":     time.Now(),
		}).Error
	}
	return &sub, nil
}

// dispatchTask 是下发给评测节点的完整任务，评测所需数据全部自带。
type dispatchTask struct {
	SubmissionID  uint           `json:"submission_id"`
	JudgeToken    string         `json:"judge_token"`
	ProblemID     uint           `json:"problem_id"`
	Language      string         `json:"language"`
	Code          string         `json:"code"`
	TimeLimitMS   int            `json:"time_limit_ms"`
	MemoryLimitMB int            `json:"memory_limit_mb"`
	CompareMode   string         `json:"compare_mode"`
	FloatEps      float64        `json:"float_eps"`
	Testcases     []dispatchCase `json:"testcases"`
}

type dispatchCase struct {
	Index int `json:"index"`
	Score int `json:"score"`
}

// JudgeResultPayload 是评测节点回传的结果。
type JudgeResultPayload struct {
	SubmissionID uint                   `json:"submission_id"`
	JudgeToken   string                 `json:"judge_token"`
	Status       string                 `json:"status"`
	Score        int                    `json:"score"`
	TimeMS       int                    `json:"time_ms"`
	MemoryKB     int                    `json:"memory_kb"`
	CompileError string                 `json:"compile_error"`
	JudgeMessage string                 `json:"judge_message"`
	Results      []model.TestcaseResult `json:"testcase_results"`
}

// dispatch 把任务推给任一可用节点；节点按配置顺序尝试，第一个接收即成功。
func (s *JudgeService) dispatch(ctx context.Context, sub *model.Submission, prob *model.Problem) error {
	nodes := s.settings.Current().JudgeNodeEndpoints
	if len(nodes) == 0 {
		return errors.New("未配置评测节点")
	}

	var cases []model.Testcase
	if err := s.db.Where("problem_id = ?", prob.ID).Order("case_index asc").Find(&cases).Error; err != nil {
		return err
	}
	items := make([]dispatchCase, 0, len(cases))
	for _, tc := range cases {
		items = append(items, dispatchCase{Index: tc.Index, Score: tc.Score})
	}

	task := dispatchTask{
		SubmissionID:  sub.ID,
		JudgeToken:    sub.JudgeToken,
		ProblemID:     prob.ID,
		Language:      sub.Language,
		Code:          sub.Code,
		TimeLimitMS:   prob.TimeLimitMS,
		MemoryLimitMB: prob.MemoryLimitMB,
		CompareMode:   prob.CompareMode,
		FloatEps:      prob.FloatEps,
		Testcases:     items,
	}
	body, err := json.Marshal(task)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var lastErr error
	for _, endpoint := range nodes {
		url := strings.TrimRight(endpoint, "/") + "/internal/v1/judge"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Sync-Secret", s.cfg.Judge.SyncSecret)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		lastErr = fmt.Errorf("节点 %s 返回 %d", endpoint, resp.StatusCode)
	}
	if lastErr == nil {
		lastErr = errors.New("没有可用节点")
	}
	return lastErr
}

// Rejudge 重置指定提交并重新投递评测任务，返回成功投递的数量。
func (s *JudgeService) Rejudge(ctx context.Context, ids []uint) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	count := 0
	for _, id := range ids {
		var sub model.Submission
		if err := s.db.First(&sub, id).Error; err != nil {
			continue
		}
		var prob model.Problem
		if err := s.db.First(&prob, sub.ProblemID).Error; err != nil {
			continue
		}
		token, err := randomHex(16)
		if err != nil {
			return count, err
		}
		res := s.db.Model(&model.Submission{}).Where("id = ?", id).Updates(map[string]any{
			"judge_token":      token,
			"status":           model.StatusPending,
			"score":            0,
			"time_ms":          0,
			"memory_kb":        0,
			"compile_error":    "",
			"judge_message":    "",
			"testcase_results": "[]",
			"judged_at":        nil,
		})
		if res.Error != nil {
			return count, res.Error
		}
		if res.RowsAffected == 0 {
			continue
		}
		sub.JudgeToken = token
		if err := s.dispatch(ctx, &sub, &prob); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// RejudgeContest 重测某场比赛的全部提交。
func (s *JudgeService) RejudgeContest(ctx context.Context, contestID uint) (int, error) {
	var ids []uint
	if err := s.db.Model(&model.Submission{}).Where("contest_id = ?", contestID).
		Order("id asc").Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	return s.Rejudge(ctx, ids)
}

// RejudgeProblem 重测某道题的全部提交。
func (s *JudgeService) RejudgeProblem(ctx context.Context, problemID uint) (int, error) {
	var ids []uint
	if err := s.db.Model(&model.Submission{}).Where("problem_id = ?", problemID).
		Order("id asc").Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	return s.Rejudge(ctx, ids)
}

// ParseIDs 解析逗号分隔的提交 ID 列表。
func ParseIDs(raw string) []uint {
	var ids []uint
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var id uint64
		if _, err := fmt.Sscanf(part, "%d", &id); err == nil && id > 0 {
			ids = append(ids, uint(id))
		}
	}
	return ids
}
