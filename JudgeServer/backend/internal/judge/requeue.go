package judge

import (
	"context"
	"time"

	"akioi/internal/config"
	"akioi/internal/model"
	"akioi/internal/queue"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// RequeueUnfinished 重新投递近期未完成的提交，用于评测进程崩溃后恢复。
func RequeueUnfinished(ctx context.Context, db *gorm.DB, cfg *config.Config, rdb *goredis.Client, within time.Duration) (int, error) {
	var subs []model.Submission
	if err := db.Where("status IN ? AND created_at >= ?",
		[]string{model.StatusPending, model.StatusJudging}, time.Now().Add(-within)).
		Order("id asc").Find(&subs).Error; err != nil {
		return 0, err
	}
	q := queue.New(rdb, cfg.Judge.QueueKey)
	count := 0
	for i := range subs {
		if err := q.Push(ctx, queue.Task{SubmissionID: subs[i].ID, Token: subs[i].JudgeToken}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
