package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"akioi/internal/config"
	"akioi/internal/database"
	"akioi/internal/judge"
	"akioi/internal/logger"
	"akioi/internal/queue"
	"akioi/internal/redis"
	"akioi/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	if err := logger.Init(cfg.LogDir, cfg.LogMaxMB); err != nil {
		panic(err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Errorf("评测进程连接数据库失败: %v", err)
		panic(err)
	}
	rdb, err := redis.Connect(cfg)
	if err != nil {
		logger.Errorf("评测进程连接 Redis 失败: %v", err)
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

	judgeSvc := service.NewJudge(db, cfg, rdb)
	count, err := judgeSvc.RequeueUnfinished(ctx, 24*time.Hour)
	if err != nil {
		logger.Errorf("重新投递未完成提交失败: %v", err)
	} else if count > 0 {
		logger.Judgef("已重新投递 %d 条未完成的提交", count)
	}

	engine := judge.New(cfg, db)
	q := queue.New(rdb, cfg.Judge.QueueKey)
	workers := cfg.Judge.Concurrency
	logger.Businessf("评测进程启动，并发度 %d，队列 %s", workers, cfg.Judge.QueueKey)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			loop(ctx, cfg, q, engine, id)
		}(i)
	}
	wg.Wait()
	logger.Businessf("评测进程退出")
}

func loop(ctx context.Context, cfg *config.Config, q *queue.Queue, engine *judge.Engine, workerID int) {
	for {
		if ctx.Err() != nil {
			return
		}
		task, err := q.Pop(ctx, 2*time.Second)
		if err != nil {
			continue
		}
		taskCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Judge.TaskTimeoutSec)*time.Second)
		if err := engine.Judge(taskCtx, task.SubmissionID, task.Token); err != nil {
			logger.Errorf("评测任务失败 submission=%d worker=%d err=%v", task.SubmissionID, workerID, err)
		}
		cancel()
	}
}
