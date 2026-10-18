package middleware

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/redis"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
)

// slidingWindowScript 以有序集合实现滑动窗口计数：清理过期成员、计数、写入本次请求。
var slidingWindowScript = goredis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
local count = redis.call('ZCARD', key)
redis.call('ZADD', key, now, ARGV[3])
redis.call('PEXPIRE', key, window)
return count
`)

// RateLimit 基于 Redis 滑动窗口限流；管理员豁免，Redis 不可用时按失败关闭处理。
func RateLimit(rdb *goredis.Client, currentLimit func() int) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := currentLimit()
		if limit <= 0 {
			c.Next()
			return
		}
		if u := httpx.CurrentUser(c); u != nil && u.IsAdmin() {
			c.Next()
			return
		}

		identity := "ip:" + c.ClientIP()
		if u := httpx.CurrentUser(c); u != nil {
			identity = "user:" + strconv.FormatUint(uint64(u.ID), 10)
		}
		key := redis.KeyRateLimitPrefix + identity
		window := time.Minute.Milliseconds()
		member := fmt.Sprintf("%d-%s", time.Now().UnixNano(), c.Request.URL.Path)

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		count, err := slidingWindowScript.Run(ctx, rdb, []string{key}, time.Now().UnixMilli(), window, member).Int()
		if err != nil {
			logger.Errorf("限流检查失败，按拒绝处理: %v", err)
			httpx.TooMany(c, httpx.CodeTooManyRequest, "服务繁忙，请稍后重试")
			c.Abort()
			return
		}
		if count >= limit {
			httpx.TooMany(c, httpx.CodeTooManyRequest, "请求过于频繁，请稍后重试")
			c.Abort()
			return
		}
		c.Next()
	}
}
