package handler

import (
	"akioi/internal/httpx"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// syncProblemTags 把题目上的标签同步进 tags / problem_tags 两张表：
// 先清掉这道题原有的关联，再按标签名逐个「取已有的或新建」，然后重建关联。
func (h *Handler) syncProblemTags(tx *gorm.DB, problemID uint, tags []string) error {
	names := model.NormalizeTags(tags)
	if err := tx.Where("problem_id = ?", problemID).Delete(&model.ProblemTag{}).Error; err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	links := make([]model.ProblemTag, 0, len(names))
	for _, name := range names {
		tag := model.Tag{Name: name}
		if err := tx.Where("name = ?", name).FirstOrCreate(&tag).Error; err != nil {
			return err
		}
		links = append(links, model.ProblemTag{ProblemID: problemID, TagID: tag.ID})
	}
	return tx.Create(&links).Error
}

// ListTags 返回所有已经在用的标签，供新建 / 编辑题目时提示。
func (h *Handler) ListTags(c *gin.Context) {
	var tags []model.Tag
	if err := h.DB.Order("name asc").Find(&tags).Error; err != nil {
		httpx.ServerError(c, "查询标签失败")
		return
	}
	items := make([]gin.H, 0, len(tags))
	for _, tag := range tags {
		items = append(items, gin.H{"id": tag.ID, "name": tag.Name})
	}
	httpx.OK(c, gin.H{"items": items, "total": len(items)})
}

// tagNamesOf 取题目当前的标签名列表。
func tagNamesOf(p *model.Problem) []string {
	return model.NormalizeTags(p.Tags())
}
