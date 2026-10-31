package handler

import (
	"fmt"
	"regexp"
	"strings"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var problemBankPrefixRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,7}$`)

type adminProblemBankItem struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	CodeLength int    `json:"code_length"`
	// Role 为空表示后台自建题库；预置题库带角色且不能删除。
	Role         string `json:"role"`
	Preset       bool   `json:"preset"`
	Remark       string `json:"remark"`
	ProblemCount int64  `json:"problem_count"`
	// SampleDisplayID 是题库的首个题号示例，前端用它展示编号规则。
	SampleDisplayID string `json:"sample_display_id"`
}

// AdminListProblemBanks 返回全部题库及各题库下的题目数量。
func (h *Handler) AdminListProblemBanks(c *gin.Context) {
	var banks []model.ProblemBank
	if err := h.DB.Order("id asc").Find(&banks).Error; err != nil {
		httpx.ServerError(c, "查询题库失败")
		return
	}

	type countRow struct {
		BankID uint
		Total  int64
	}
	var rows []countRow
	if err := h.DB.Model(&model.Problem{}).
		Select("bank_id, count(*) as total").
		Group("bank_id").
		Scan(&rows).Error; err != nil {
		httpx.ServerError(c, "查询题库失败")
		return
	}
	counts := make(map[uint]int64, len(rows))
	for _, row := range rows {
		counts[row.BankID] = row.Total
	}

	items := make([]adminProblemBankItem, 0, len(banks))
	for i := range banks {
		b := &banks[i]
		items = append(items, adminProblemBankItem{
			ID:              b.ID,
			Name:            b.Name,
			Prefix:          b.Prefix,
			CodeLength:      b.CodeLength,
			Role:            b.Role,
			Preset:          b.IsPreset(),
			Remark:          b.Remark,
			ProblemCount:    counts[b.ID],
			SampleDisplayID: service.FormatProblemDisplayID(b, 0),
		})
	}
	httpx.OK(c, gin.H{"items": items, "total": len(items)})
}

type adminProblemBankRequest struct {
	Name       *string `json:"name"`
	Prefix     *string `json:"prefix"`
	CodeLength *int    `json:"code_length"`
	Remark     *string `json:"remark"`
}

// AdminCreateProblemBank 新建题库；前缀与题号长度决定该题库的编号规则。
func (h *Handler) AdminCreateProblemBank(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req adminProblemBankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}

	name := ""
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	if name == "" {
		httpx.BadRequest(c, "题库名称不能为空")
		return
	}
	if len([]rune(name)) > 64 {
		httpx.BadRequest(c, "题库名称不能超过 64 个字符")
		return
	}

	prefix := ""
	if req.Prefix != nil {
		prefix = strings.TrimSpace(*req.Prefix)
	}
	if !problemBankPrefixRe.MatchString(prefix) {
		httpx.BadRequest(c, "题库前缀需以字母开头，且只含字母或数字，例如 A")
		return
	}

	codeLength := 4
	if req.CodeLength != nil {
		codeLength = *req.CodeLength
	}
	if err := validProblemBankCodeLength(prefix, codeLength); err != "" {
		httpx.BadRequest(c, err)
		return
	}

	var exists int64
	if err := h.DB.Model(&model.ProblemBank{}).Where("prefix = ?", prefix).Count(&exists).Error; err != nil {
		httpx.ServerError(c, "创建题库失败")
		return
	}
	if exists > 0 {
		httpx.Conflict(c, "题库前缀已存在")
		return
	}

	bank := model.ProblemBank{Name: name, Prefix: prefix, CodeLength: codeLength}
	if req.Remark != nil {
		bank.Remark = strings.TrimSpace(*req.Remark)
	}
	if err := h.DB.Create(&bank).Error; err != nil {
		httpx.ServerError(c, "创建题库失败")
		return
	}
	logger.Adminf("管理员 %s 创建题库 %s（前缀 %s，题号 %d 位）ip=%s", u.Username, bank.Name, bank.Prefix, bank.CodeLength, c.ClientIP())
	httpx.OK(c, gin.H{"bank": gin.H{"id": bank.ID, "prefix": bank.Prefix}})
}

// AdminUpdateProblemBank 更新题库。预置题库只能改前缀与题号长度，改完按原序号重排题目；
// 自建题库的名称与备注可改，前缀与题号长度决定历史题号格式，创建后不可修改。
func (h *Handler) AdminUpdateProblemBank(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var bank model.ProblemBank
	if err := h.DB.First(&bank, id).Error; err != nil {
		httpx.NotFound(c, "题库不存在")
		return
	}
	var req adminProblemBankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}

	updates := map[string]any{}
	prefix := bank.Prefix
	codeLength := bank.CodeLength
	renumber := false

	if bank.IsPreset() {
		// 预置题库只允许调整前缀与题号长度，名称与备注保持初始化的取值
		if req.Prefix != nil {
			prefix = strings.TrimSpace(*req.Prefix)
			if !problemBankPrefixRe.MatchString(prefix) {
				httpx.BadRequest(c, "题库前缀需以字母开头，且只含字母或数字，例如 A")
				return
			}
		}
		if req.CodeLength != nil {
			codeLength = *req.CodeLength
		}
		if errText := validProblemBankCodeLength(prefix, codeLength); errText != "" {
			httpx.BadRequest(c, errText)
			return
		}
		if prefix != bank.Prefix {
			var exists int64
			if err := h.DB.Model(&model.ProblemBank{}).Where("prefix = ? AND id <> ?", prefix, bank.ID).Count(&exists).Error; err != nil {
				httpx.ServerError(c, "更新题库失败")
				return
			}
			if exists > 0 {
				httpx.Conflict(c, "题库前缀已存在")
				return
			}
		}
		if prefix != bank.Prefix || codeLength != bank.CodeLength {
			updates["prefix"] = prefix
			updates["code_length"] = codeLength
			renumber = true
		}
	} else {
		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				httpx.BadRequest(c, "题库名称不能为空")
				return
			}
			if len([]rune(name)) > 64 {
				httpx.BadRequest(c, "题库名称不能超过 64 个字符")
				return
			}
			updates["name"] = name
		}
		if req.Prefix != nil && strings.TrimSpace(*req.Prefix) != bank.Prefix {
			httpx.BadRequest(c, "题库前缀创建后不可修改")
			return
		}
		if req.CodeLength != nil && *req.CodeLength != bank.CodeLength {
			httpx.BadRequest(c, "题号长度创建后不可修改")
			return
		}
		if req.Remark != nil {
			updates["remark"] = strings.TrimSpace(*req.Remark)
		}
	}

	if len(updates) > 0 || renumber {
		err := h.DB.Transaction(func(tx *gorm.DB) error {
			if len(updates) > 0 {
				if err := tx.Model(&model.ProblemBank{}).Where("id = ?", bank.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
			if !renumber {
				return nil
			}
			return renumberProblemBank(tx, bank.ID, bank.Prefix, prefix, codeLength)
		})
		if err != nil {
			logger.Errorf("更新题库 %d 失败: %v", bank.ID, err)
			httpx.ServerError(c, "更新题库失败")
			return
		}
	}
	logger.Adminf("管理员 %s 更新题库 %s（id=%d，前缀 %s，题号 %d 位）ip=%s", u.Username, bank.Name, bank.ID, prefix, codeLength, c.ClientIP())
	httpx.OK(c, gin.H{"bank": gin.H{"id": bank.ID, "prefix": prefix, "code_length": codeLength}})
}

// renumberProblemBank 按题号里的原序号重排题库内所有题号：先写入临时值再写最终值，
// 避免同一题库内新旧题号在唯一索引上互相占用。已删除的题目一并重排，保证唯一索引不冲突。
func renumberProblemBank(tx *gorm.DB, bankID uint, oldPrefix, prefix string, codeLength int) error {
	var problems []model.Problem
	if err := tx.Unscoped().Select("id, display_id").Where("bank_id = ?", bankID).Order("id asc").Find(&problems).Error; err != nil {
		return err
	}
	if len(problems) == 0 {
		return nil
	}
	oldIDs := make([]string, 0, len(problems))
	for _, p := range problems {
		oldIDs = append(oldIDs, p.DisplayID)
	}
	target := &model.ProblemBank{Prefix: prefix, CodeLength: codeLength}
	newIDs := service.RenumberProblemDisplayIDs(target, oldPrefix, oldIDs)
	for _, p := range problems {
		// 临时题号以 ~ 开头，题号前缀只能是字母或数字，不会与正式题号冲突
		if err := tx.Model(&model.Problem{}).Where("id = ?", p.ID).Update("display_id", fmt.Sprintf("~%d", p.ID)).Error; err != nil {
			return err
		}
	}
	for i, p := range problems {
		if err := tx.Model(&model.Problem{}).Where("id = ?", p.ID).Update("display_id", newIDs[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// validProblemBankCodeLength 校验题号长度，返回空串表示通过。
func validProblemBankCodeLength(prefix string, codeLength int) string {
	if codeLength < 1 || codeLength > 10 {
		return "题号长度需在 1 到 10 位之间"
	}
	if len(prefix)+codeLength > model.ProblemBankMaxCodeLength {
		return fmt.Sprintf("题库前缀与题号长度之和不能超过 %d 位", model.ProblemBankMaxCodeLength)
	}
	return ""
}

// AdminDeleteProblemBank 删除空题库；预置题库不能删除，题库内仍有题目时也拒绝删除。
func (h *Handler) AdminDeleteProblemBank(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var bank model.ProblemBank
	if err := h.DB.First(&bank, id).Error; err != nil {
		httpx.NotFound(c, "题库不存在")
		return
	}
	if bank.IsPreset() {
		httpx.Conflict(c, "预置题库不能删除")
		return
	}
	var total int64
	if err := h.DB.Model(&model.Problem{}).Where("bank_id = ?", bank.ID).Count(&total).Error; err != nil {
		httpx.ServerError(c, "删除题库失败")
		return
	}
	if total > 0 {
		httpx.Conflict(c, "题库下还有题目，请先删除或移动到其他题库")
		return
	}
	// 软删除会把行留在表里，先释放前缀，避免同名前缀被永久占用而无法再建题库。
	released := fmt.Sprintf("~%d", bank.ID)
	if len(released) > 8 {
		released = released[:8]
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ProblemBank{}).Where("id = ?", bank.ID).Update("prefix", released).Error; err != nil {
			return err
		}
		return tx.Delete(&model.ProblemBank{}, bank.ID).Error
	}); err != nil {
		httpx.ServerError(c, "删除题库失败")
		return
	}
	logger.Adminf("管理员 %s 删除题库 %s（id=%d）ip=%s", u.Username, bank.Name, bank.ID, c.ClientIP())
	httpx.OK(c, gin.H{"deleted": true})
}
