// Package redis 提供评测进程使用的 Redis 连接。
package redis

import (
	"context"
	"fmt"
	"time"

	"akioi/internal/config"

	"github.com/redis/go-redis/v9"
)

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
