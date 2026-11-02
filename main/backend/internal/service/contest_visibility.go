package service

import (
	"time"

	"akioi/internal/logger"
	"akioi/internal/model"

	"gorm.io/gorm"
)

// PublishContestProblems 把已经开始比赛里的隐藏题改成公开。
//
// 比赛开始前隐藏题只有管理员能看到，比赛一开始参赛者就要能打开题目，
// 所以统一在这里放开可见性。函数是幂等的，可以按固定间隔反复执行。
func PublishContestProblems(db *gorm.DB) error {
	var ids []uint
	if err := db.Table("contest_problems").
		Joins("JOIN contests ON contests.id = contest_problems.contest_id").
		Where("contests.start_at <= ? AND contests.deleted_at IS NULL", time.Now()).
		Pluck("DISTINCT contest_problems.problem_id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	result := db.Model(&model.Problem{}).
		Where("id IN ? AND visibility = ?", ids, model.ProblemVisibilityHidden).
		Update("visibility", model.ProblemVisibilityPublic)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		logger.Businessf("比赛开始，把 %d 道隐藏题改为公开", result.RowsAffected)
	}
	return nil
}
