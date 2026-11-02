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

// convertMyProblemsMaxBatch 是一次迁移的题目数量上限，避免一次请求耗时过长。
const convertMyProblemsMaxBatch = 100

type convertMyProblemsRequest struct {
	ProblemIDs []uint `json:"problem_ids"`
	Target     string `json:"target"`
	TeamID     uint   `json:"team_id"`
}

// ConvertMyProblems 在个人题目与团队题目之间迁移归属：
//   - target=team：把自己上传的个人题目导入团队，题目变成团队题目；
//   - target=personal：把自己上传的团队题目取回为个人题目。
//
// 两个方向都会换到对应题库（团队题库 / 个人题库）并重新分配题号，
// 团队题目取回为个人题目时要求题目作者就是当前用户。
func (h *Handler) ConvertMyProblems(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req convertMyProblemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "参数格式不正确")
		return
	}
	target := strings.ToLower(strings.TrimSpace(req.Target))
	if target != "team" && target != "personal" {
		httpx.BadRequest(c, "切换目标只能是团队或个人")
		return
	}
	ids := uniqueProblemIDs(req.ProblemIDs)
	if len(ids) == 0 {
		httpx.BadRequest(c, "请先选择要迁移的题目")
		return
	}
	if len(ids) > convertMyProblemsMaxBatch {
		httpx.BadRequest(c, fmt.Sprintf("一次最多迁移 %d 道题目", convertMyProblemsMaxBatch))
		return
	}

	var team *model.Team
	if target == "team" {
		if req.TeamID == 0 {
			httpx.BadRequest(c, "请选择要导入的团队")
			return
		}
		var ok bool
		team, ok = h.requireTeamMember(c, req.TeamID)
		if !ok {
			return
		}
	}

	result := make([]gin.H, 0, len(ids))
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var problems []model.Problem
		if err := tx.Where("id IN ?", ids).Find(&problems).Error; err != nil {
			return err
		}
		if len(problems) != len(ids) {
			return fmt.Errorf("部分题目不存在或已被删除")
		}
		for i := range problems {
			p := &problems[i]
			if !h.canManageProblem(u, p) {
				return fmt.Errorf("题目 %s 不归你管理", p.DisplayID)
			}
			switch target {
			case "team":
				if p.TeamID != 0 {
					return fmt.Errorf("题目 %s 已经是团队题目", p.DisplayID)
				}
				if p.OwnerID != u.ID && p.CreatedBy != u.ID {
					return fmt.Errorf("题目 %s 不是你上传的，不能导入团队", p.DisplayID)
				}
			case "personal":
				if p.TeamID == 0 {
					return fmt.Errorf("题目 %s 已经是个人题目", p.DisplayID)
				}
				if p.CreatedBy != u.ID {
					return fmt.Errorf("题目 %s 的作者不是你，不能转为个人题目", p.DisplayID)
				}
				if h.teamMembership(u.ID, p.TeamID) == nil {
					return fmt.Errorf("你已不在题目 %s 所属的团队里", p.DisplayID)
				}
			}
		}

		var bank *model.ProblemBank
		var err error
		if target == "team" {
			bank, err = h.problemBankByRole(model.ProblemBankRoleTeam)
		} else {
			bank, err = h.problemBankByRole(model.ProblemBankRolePersonal)
		}
		if err != nil {
			return err
		}

		for i := range problems {
			p := &problems[i]
			displayID, err := h.allocateProblemDisplayIDTx(tx, bank)
			if err != nil {
				return err
			}
			teamID, ownerID := uint(0), uint(0)
			if target == "team" {
				teamID = team.ID
			} else {
				ownerID = u.ID
			}
			updates := map[string]any{
				"team_id":    teamID,
				"owner_id":   ownerID,
				"bank_id":    bank.ID,
				"display_id": displayID,
				"access":     defaultProblemAccess(teamID, ownerID),
			}
			if err := tx.Model(&model.Problem{}).Where("id = ?", p.ID).Updates(updates).Error; err != nil {
				return err
			}
			result = append(result, gin.H{
				"id":         p.ID,
				"display_id": displayID,
				"title":      p.Title,
			})
		}
		return nil
	})
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}

	if target == "team" {
		logger.Businessf("用户 %s 把 %d 道个人题目导入团队 %s", u.Username, len(result), team.Name)
	} else {
		logger.Businessf("用户 %s 把 %d 道团队题目转为个人题目", u.Username, len(result))
	}

	message := "已转为个人题目"
	if target == "team" {
		message = "已导入团队"
	}
	httpx.OK(c, gin.H{
		"target":          target,
		"converted_count": len(result),
		"items":           result,
		"message":         message,
	})
}
