package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"akioi/internal/agent"
	"akioi/internal/config"
	"akioi/internal/database"
	"akioi/internal/judge"
	"akioi/internal/logger"
	"akioi/internal/model"
	"akioi/internal/queue"
	"akioi/internal/redis"
	"akioi/internal/report"

	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	if err := logger.Init(cfg.LogDir, cfg.LogMaxMB); err != nil {
		panic(err)
	}

	// 本地数据库与 Redis：评测节点自成一体，不依赖主站实例。
	db, err := database.Connect(cfg)
	if err != nil {
		logger.Errorf("评测进程连接本地数据库失败: %v", err)
		panic(err)
	}
	rdb, err := redis.Connect(cfg)
	if err != nil {
		logger.Errorf("评测进程连接本地 Redis 失败: %v", err)
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sandbox := judge.NewSandbox(&cfg.Judge)
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := sandbox.CheckAvailable(checkCtx); err != nil {
		logger.Errorf("沙箱不可用，评测进程退出: %v", err)
		panic(err)
	}
	if cfg.Judge.HostTestdataDir == "" {
		logger.Errorf("HOST_TESTDATA_DIR 未配置，评测将直接使用 TESTDATA_DIR=%s，容器内运行时必须改为宿主机绝对路径", cfg.Judge.TestdataDir)
	}
	if cfg.Judge.HostWorkDir == "" {
		logger.Errorf("HOST_WORK_DIR 未配置，评测将直接使用 WORK_DIR=%s，容器内运行时必须改为宿主机绝对路径", cfg.Judge.WorkDir)
	}

	engine := judge.New(cfg)
	q := queue.New(rdb, cfg.Judge.QueueKey)
	reporter := report.New(cfg)
	workers := cfg.Judge.Concurrency
	logger.Businessf("评测进程启动，并发度 %d，队列 %s，主站 %s", workers, cfg.Judge.QueueKey, cfg.MainBaseURL)

	requeuePending(ctx, db, q)

	// 控制面：接收主站推送的测试数据与评测任务；未配置密钥时不监听。
	if cfg.Judge.SyncSecret == "" {
		logger.Errorf("JUDGE_SYNC_SECRET 未配置，控制面接口未启用")
	} else {
		go agent.New(cfg, db, q).Run(ctx)
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			loop(ctx, cfg, db, q, engine, reporter, id)
		}(i)
	}
	wg.Wait()
	logger.Businessf("评测进程退出")
}

// requeuePending 把重启前未完成的任务重新入队，避免任务永久丢失。
func requeuePending(ctx context.Context, db *gorm.DB, q *queue.Queue) {
	var jobs []model.JudgeJob
	if err := db.Where("status IN ?", []string{"pending", "running"}).Find(&jobs).Error; err != nil {
		logger.Errorf("查询未完成任务失败: %v", err)
		return
	}
	for _, job := range jobs {
		if err := db.Model(&job).Update("status", "pending").Error; err != nil {
			continue
		}
		if err := q.Push(ctx, queue.Task{JobID: job.ID}); err != nil {
			logger.Errorf("重新入队任务 %d 失败: %v", job.ID, err)
		}
	}
	if len(jobs) > 0 {
		logger.Judgef("已重新入队 %d 条未完成任务", len(jobs))
	}
}

func loop(ctx context.Context, cfg *config.Config, db *gorm.DB, q *queue.Queue,
	engine *judge.Engine, reporter *report.Reporter, workerID int) {

	for {
		if ctx.Err() != nil {
			return
		}
		task, err := q.Pop(ctx, 2*time.Second)
		if err != nil {
			continue
		}

		var job model.JudgeJob
		if err := db.First(&job, task.JobID).Error; err != nil {
			logger.Errorf("任务 %d 不存在", task.JobID)
			continue
		}
		if job.Status == "done" {
			continue
		}

		var payload model.JudgeTask
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			logger.Errorf("任务 %d 内容损坏: %v", job.ID, err)
			_ = db.Model(&job).Update("status", "failed").Error
			continue
		}

		_ = db.Model(&job).Update("status", "running").Error
		taskCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Judge.TaskTimeoutSec)*time.Second)
		result := engine.Judge(taskCtx, &payload)
		cancel()

		if raw, err := json.Marshal(result); err == nil {
			_ = db.Model(&job).Updates(map[string]any{"result": string(raw), "status": "done"}).Error
		}

		if err := sendResult(ctx, reporter, result); err != nil {
			logger.Errorf("回传评测结果失败 submission=%d worker=%d: %v", payload.SubmissionID, workerID, err)
			_ = db.Model(&job).Update("status", "failed").Error
			continue
		}
		_ = db.Model(&job).Update("synced", true).Error
	}
}

// sendResult 回传结果，失败时重试三次；结果本身已经落在本地库，不会丢。
func sendResult(ctx context.Context, reporter *report.Reporter, result *model.JudgeResult) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		lastErr = reporter.Send(sendCtx, result)
		cancel()
		if lastErr == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
	}
	return lastErr
}
