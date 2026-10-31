package handler

import (
	"regexp"
	"strings"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

var problemBankPrefixRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,7}$`)

type adminProblemBankItem struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Prefix       string `json:"prefix"`
	CodeLength   int    `json:"code_length"`
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
	if codeLength < 1 || codeLength > 10 {
		httpx.BadRequest(c, "题号长度需在 1 到 10 位之间")
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

// AdminUpdateProblemBank 更新题库名称与备注；前缀和题号长度决定历史题号格式，创建后不可修改。
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

	if len(updates) > 0 {
		if err := h.DB.Model(&model.ProblemBank{}).Where("id = ?", bank.ID).Updates(updates).Error; err != nil {
			httpx.ServerError(c, "更新题库失败")
			return
		}
	}
	logger.Adminf("管理员 %s 更新题库 %s（id=%d）ip=%s", u.Username, bank.Name, bank.ID, c.ClientIP())
	httpx.OK(c, gin.H{"bank": gin.H{"id": bank.ID, "prefix": bank.Prefix}})
}

// AdminDeleteProblemBank 删除空题库；题库内仍有题目时拒绝删除。
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
	var total int64
	if err := h.DB.Model(&model.Problem{}).Where("bank_id = ?", bank.ID).Count(&total).Error; err != nil {
		httpx.ServerError(c, "删除题库失败")
		return
	}
	if total > 0 {
		httpx.Conflict(c, "题库下还有题目，请先删除或移动到其他题库")
		return
	}
	if err := h.DB.Delete(&model.ProblemBank{}, bank.ID).Error; err != nil {
		httpx.ServerError(c, "删除题库失败")
		return
	}
	logger.Adminf("管理员 %s 删除题库 %s（id=%d）ip=%s", u.Username, bank.Name, bank.ID, c.ClientIP())
	httpx.OK(c, gin.H{"deleted": true})
}
