// Package queue 实现基于 Redis 列表的评测任务队列。
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Task 是下发给评测进程的任务，只携带本地任务 ID。
type Task struct {
	JobID uint `json:"job_id"`
}

var ErrEmpty = errors.New("队列为空")

type Queue struct {
	rdb *redis.Client
	key string
}

func New(rdb *redis.Client, key string) *Queue {
	return &Queue{rdb: rdb, key: key}
}

func (q *Queue) Push(ctx context.Context, task Task) error {
	b, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return q.rdb.LPush(ctx, q.key, b).Err()
}

// Pop 阻塞等待任务，超时返回 ErrEmpty。
func (q *Queue) Pop(ctx context.Context, timeout time.Duration) (*Task, error) {
	res, err := q.rdb.BRPop(ctx, timeout, q.key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) || errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrEmpty
		}
		return nil, err
	}
	if len(res) != 2 {
		return nil, ErrEmpty
	}
	var task Task
	if err := json.Unmarshal([]byte(res[1]), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (q *Queue) Length(ctx context.Context) (int64, error) {
	return q.rdb.LLen(ctx, q.key).Result()
}
