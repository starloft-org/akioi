package handler

import (
	"fmt"
	"strings"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminMoveProblemsBankMaxBatch 是一次批量迁移的题目数量上限，避免一次请求耗时过长。
const AdminMoveProblemsBankMaxBatch = 500

type moveProblemsBankRequest struct {
	ProblemIDs []uint `json:"problem_ids"`
	BankID     uint   `json:"bank_id"`
}

// AdminMoveProblemsBank 把选中的题目迁移到目标题库，并在新题库里重新分配题号。
// 后台自建题库与主库都可以作为目标；团队题库、个人题库只放对应归属的题目，不能选。
// 个人题目迁到公共题库时会转成公共题目（清空归属、访问权限改回公开）。
func (h *Handler) AdminMoveProblemsBank(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req moveProblemsBankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "参数格式不正确")
		return
	}
	ids := uniqueProblemIDs(req.ProblemIDs)
	if len(ids) == 0 {
		httpx.BadRequest(c, "请先选择要迁移的题目")
		return
	}
	if len(ids) > AdminMoveProblemsBankMaxBatch {
		httpx.BadRequest(c, fmt.Sprintf("一次最多迁移 %d 道题目", AdminMoveProblemsBankMaxBatch))
		return
	}
	if req.BankID == 0 {
		httpx.BadRequest(c, "请选择目标题库")
		return
	}

	var bank model.ProblemBank
	if err := h.DB.First(&bank, req.BankID).Error; err != nil {
		httpx.BadRequest(c, "目标题库不存在")
		return
	}
	if bank.IsPreset() && bank.Role != model.ProblemBankRolePrimary {
		httpx.BadRequest(c, "团队题库、个人题库只放对应归属的题目，请选择公共题库作为目标")
		return
	}

	var problems []model.Problem
	if err := h.DB.Where("id IN ?", ids).Find(&problems).Error; err != nil {
		httpx.ServerError(c, "查询题目失败")
		return
	}
	if len(problems) == 0 {
		httpx.NotFound(c, "题目不存在或已删除")
		return
	}

	// 团队题目跟着团队走，不能改成公共题目；个人题目迁移过来则顺手转成公共题目
	teams := make([]string, 0)
	skipped := 0
	for i := range problems {
		if problems[i].Kind() == model.ProblemKindTeam {
			teams = append(teams, problems[i].DisplayID)
			continue
		}
		if problems[i].BankID == bank.ID {
			skipped++
		}
	}
	if len(teams) > 0 {
		httpx.BadRequest(c, fmt.Sprintf("团队题目不能迁移到公共题库，请先在编辑页把归属改成公共：%s", strings.Join(teams, "、")))
		return
	}

	moved := make([]gin.H, 0, len(problems))
	converted := 0
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		for i := range problems {
			p := &problems[i]
			if p.BankID == bank.ID {
				continue
			}
			displayID, err := h.allocateProblemDisplayIDTx(tx, &bank)
			if err != nil {
				return err
			}
			updates := map[string]any{"bank_id": bank.ID, "display_id": displayID}
			// 个人题目落到公共题库后就变成公共题目：清空归属，访问权限回到公开
			fromOwner := p.OwnerID
			if p.IsPersonalProblem() {
				updates["owner_id"] = uint(0)
				updates["team_id"] = uint(0)
				updates["access"] = model.ProblemAccessPublic
				converted++
			}
			if err := tx.Model(&model.Problem{}).Where("id = ?", p.ID).
				Updates(updates).Error; err != nil {
				return err
			}
			moved = append(moved, gin.H{
				"id":             p.ID,
				"old_display_id": p.DisplayID,
				"display_id":     displayID,
				"from_owner_id":  fromOwner,
			})
		}
		return nil
	})
	if err != nil {
		httpx.ServerError(c, "迁移题目失败："+err.Error())
		return
	}

	if len(moved) > 0 {
		logger.Adminf("管理员 %s 把 %d 道题目迁移到题库 %s（id=%d） ip=%s",
			u.Username, len(moved), bank.Name, bank.ID, c.ClientIP())
	}
	httpx.OK(c, gin.H{
		"bank_id":         bank.ID,
		"bank_name":       bank.Name,
		"moved":           moved,
		"moved_count":     len(moved),
		"skipped_count":   skipped,
		"converted_count": converted,
	})
}

// uniqueProblemIDs 去掉重复和 0 的题目 ID，保持传入顺序。
func uniqueProblemIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint]bool, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
