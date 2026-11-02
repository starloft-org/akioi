package database

import (
	"akioi/internal/logger"
	"akioi/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// backfillProblemTags 把存量题目 JSON 里的标签写进 tags / problem_tags 表。
// 幂等：已在表里的标签直接复用，已存在的关联忽略。
func backfillProblemTags(db *gorm.DB) error {
	var problems []model.Problem
	if err := db.Select("id", "tags").Find(&problems).Error; err != nil {
		return err
	}
	var before int64
	if err := db.Model(&model.Tag{}).Count(&before).Error; err != nil {
		return err
	}
	tagIDs := make(map[string]uint)
	links := make([]model.ProblemTag, 0, len(problems))
	for _, p := range problems {
		for _, name := range model.NormalizeTags(p.Tags()) {
			id, ok := tagIDs[name]
			if !ok {
				tag := model.Tag{Name: name}
				if err := db.Where("name = ?", name).FirstOrCreate(&tag).Error; err != nil {
					return err
				}
				id = tag.ID
				tagIDs[name] = id
			}
			links = append(links, model.ProblemTag{ProblemID: p.ID, TagID: id})
		}
	}
	for start := 0; start < len(links); start += 500 {
		end := start + 500
		if end > len(links) {
			end = len(links)
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(links[start:end]).Error; err != nil {
			return err
		}
	}
	if before == 0 && len(tagIDs) > 0 {
		logger.Businessf("题目标签表初始化完成：%d 个标签，%d 条题目关联", len(tagIDs), len(links))
	}
	return nil
}
