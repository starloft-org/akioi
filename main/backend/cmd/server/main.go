package main

import (
	"context"
	"time"

	"akioi/internal/config"
	"akioi/internal/database"
	"akioi/internal/handler"
	"akioi/internal/logger"
	"akioi/internal/migrate"
	"akioi/internal/redis"
	"akioi/internal/router"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	if err := logger.Init(cfg.LogDir, cfg.LogMaxMB); err != nil {
		panic(err)
	}
	if err := migrate.Run(database.MigrateDSN(cfg)); err != nil {
		logger.Errorf("数据库迁移失败: %v", err)
		panic(err)
	}
	db, err := database.Connect(cfg)
	if err != nil {
		logger.Errorf("连接数据库失败: %v", err)
		panic(err)
	}
	if err := database.AutoMigrate(db); err != nil {
		logger.Errorf("数据库表结构补齐失败: %v", err)
		panic(err)
	}
	if err := database.Seed(db, cfg); err != nil {
		logger.Errorf("初始数据写入失败: %v", err)
		panic(err)
	}
	settings, err := service.NewSettingService(db)
	if err != nil {
		logger.Errorf("加载系统设置失败: %v", err)
		panic(err)
	}
	logger.Businessf("启动 %s，环境 %s", settings.Current().AppName, cfg.Env)
	rdb, err := redis.Connect(cfg)
	if err != nil {
		logger.Errorf("连接 Redis 失败: %v", err)
		panic(err)
	}

	gin.SetMode(gin.ReleaseMode)
	if cfg.Env == "development" {
		gin.SetMode(gin.DebugMode)
	}
	h := handler.New(db, cfg, rdb, settings)
	r := router.Setup(h, cfg, db, rdb)

	// 测试数据只在主站保存：每 2 分钟把有变更、还没同步成功的题目补推到各评测节点
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			h.Testdata.SyncPending(context.Background())
		}
	}()

	// 比赛到点后把题目里的隐藏题放开：每 30 秒检查一次，失败只记日志
	go func() {
		if err := service.PublishContestProblems(db); err != nil {
			logger.Errorf("比赛题目公开检查失败: %v", err)
		}
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := service.PublishContestProblems(db); err != nil {
				logger.Errorf("比赛题目公开检查失败: %v", err)
			}
		}
	}()

	logger.Businessf("HTTP 服务监听 %s", cfg.ServerAddr)
	if err := r.Run(cfg.ServerAddr); err != nil {
		logger.Errorf("HTTP 服务退出: %v", err)
		panic(err)
	}
}
