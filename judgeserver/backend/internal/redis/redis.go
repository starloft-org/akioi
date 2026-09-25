// Package redis 提供评测进程使用的 Redis 连接与任务锁键约定。
package redis

import (
	"context"
	"fmt"
	"time"

	"akioi/internal/config"

	"github.com/redis/go-redis/v9"
)

// KeyJudgeLockPrefix 为单次评测任务的占用锁前缀，完整键为 {前缀}{提交ID}:{judge_token}。
const KeyJudgeLockPrefix = "akioi:judge:lock:"

// Connect 建立 Redis 连接并探测可用性。
func Connect(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}
	return client, nil
}
