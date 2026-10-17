// Package redis 提供 Redis 连接与业务缓存/队列的键约定。
package redis

import (
	"context"
	"fmt"
	"time"

	"akioi/internal/config"

	"github.com/redis/go-redis/v9"
)

// 会话与限流使用的键前缀。
const (
	KeySessionPrefix     = "akioi:session:"
	KeyRateLimitPrefix   = "akioi:ratelimit:"
	KeySubmitLockPrefix  = "akioi:submitlock:"
	KeyWeChatStatePrefix = "akioi:wxstate:"
	KeySMSCodePrefix      = "akioi:sms:code:"
	KeySMSCooldownPrefix  = "akioi:sms:cooldown:"
	KeySMSAttemptsPrefix  = "akioi:sms:attempts:"
	// KeyJudgeRunningPrefix 记录在线评测任务，用于统计运行中数量。
	KeyJudgeRunningPrefix = "akioi:judge:running:"
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
