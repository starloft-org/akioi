package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"akioi/internal/config"
	"akioi/internal/model"
	"akioi/internal/queue"
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
	db  *gorm.DB
	cfg *config.Config
	rdb *goredis.Client
}

func NewJudge(db *gorm.DB, cfg *config.Config, rdb *goredis.Client) *JudgeService {
	return &JudgeService{db: db, cfg: cfg, rdb: rdb}
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

	if s.cfg.SubmitIntervalSec > 0 {
		key := fmt.Sprintf("%s%d:%d", redis.KeySubmitLockPrefix, in.UserID, in.ProblemID)
		ok, err := s.rdb.SetNX(ctx, key, time.Now().Unix(), time.Duration(s.cfg.SubmitIntervalSec)*time.Second).Result()
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
	if err := queue.New(s.rdb, s.cfg.Judge.QueueKey).Push(ctx, queue.Task{SubmissionID: sub.ID, Token: token}); err != nil {
		return nil, fmt.Errorf("投递评测任务失败: %w", err)
	}
	return &sub, nil
}

// Rejudge 重置指定提交并重新投递评测任务，返回成功投递的数量。
func (s *JudgeService) Rejudge(ctx context.Context, ids []uint) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	q := queue.New(s.rdb, s.cfg.Judge.QueueKey)
	count := 0
	for _, id := range ids {
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
		if err := q.Push(ctx, queue.Task{SubmissionID: id, Token: token}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// RequeueUnfinished 重新投递未完成的提交，用于评测进程崩溃后恢复。
func (s *JudgeService) RequeueUnfinished(ctx context.Context, within time.Duration) (int, error) {
	var subs []model.Submission
	if err := s.db.Where("status IN ? AND created_at >= ?",
		[]string{model.StatusPending, model.StatusJudging}, time.Now().Add(-within)).
		Order("id asc").Find(&subs).Error; err != nil {
		return 0, err
	}
	q := queue.New(s.rdb, s.cfg.Judge.QueueKey)
	count := 0
	for i := range subs {
		if err := q.Push(ctx, queue.Task{SubmissionID: subs[i].ID, Token: subs[i].JudgeToken}); err != nil {
			return count, err
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
