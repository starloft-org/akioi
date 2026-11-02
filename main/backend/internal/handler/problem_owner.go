package handler

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"akioi/internal/httpx"
	"akioi/internal/model"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

// 题目归属与题库 -----------------------------------------------------------------

// problemBankByRole 按预置角色取题库：团队题用团队库、个人题用个人库。
func (h *Handler) problemBankByRole(role string) (*model.ProblemBank, error) {
	var bank model.ProblemBank
	if err := h.DB.Where("preset_role = ?", role).Order("id asc").First(&bank).Error; err != nil {
		return nil, fmt.Errorf("预置题库缺失，请检查题库管理中的主库、团队库与个人库")
	}
	return &bank, nil
}

// problemBankForOwner 按归属取题库：团队题用团队库，个人题用个人库，
// 公共题目使用主库（预置的主库）。
func (h *Handler) problemBankForOwner(teamID, ownerID uint) (*model.ProblemBank, error) {
	switch {
	case teamID > 0:
		return h.problemBankByRole(model.ProblemBankRoleTeam)
	case ownerID > 0:
		return h.problemBankByRole(model.ProblemBankRolePersonal)
	default:
		return h.resolveProblemBank(nil)
	}
}

// validProblemAccessFor 校验访问权限与归属是否匹配：公共题目只能公开。
func validProblemAccessFor(teamID, ownerID uint, access int) bool {
	if !model.ValidProblemAccess(access) {
		return false
	}
	if teamID == 0 && ownerID == 0 {
		return access == model.ProblemAccessPublic
	}
	return true
}

// defaultProblemAccess 返回归属对应的默认访问权限：公共题目公开，
// 团队题目与个人题目默认仅所属成员/本人可访问。
func defaultProblemAccess(teamID, ownerID uint) int {
	if teamID == 0 && ownerID == 0 {
		return model.ProblemAccessPublic
	}
	return model.ProblemAccessMembers
}

// canManageProblem 判断用户能否管理该题：个人题仅创建者，团队题仅队长，公共题只有管理员。
func (h *Handler) canManageProblem(u *model.User, p *model.Problem) bool {
	if u == nil {
		return false
	}
	if u.IsAdmin() {
		return true
	}
	if p.IsTeamProblem() {
		m := h.teamMembership(u.ID, p.TeamID)
		return m != nil && m.Role == model.TeamRoleOwner
	}
	return p.OwnerID > 0 && p.OwnerID == u.ID
}

// 测试数据 ---------------------------------------------------------------------

// uploadProblemTestdata 解析上传的测试数据并整体替换，支持单 zip 与多文件两种形式。
func (h *Handler) uploadProblemTestdata(c *gin.Context, p *model.Problem) {
	form, err := c.MultipartForm()
	if err != nil {
		httpx.BadRequest(c, "解析上传内容失败")
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		httpx.BadRequest(c, "缺少测试数据文件")
		return
	}
	maxBytes := int64(h.Settings.Current().MaxTestdataMB) * 1024 * 1024

	var data map[string][]byte
	if len(files) == 1 && strings.EqualFold(filepath.Ext(files[0].Filename), ".zip") {
		content, err := readUploadedFile(files[0], maxBytes)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		data, err = service.ExtractZip(content)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
	} else {
		data = make(map[string][]byte, len(files))
		for _, fh := range files {
			content, err := readUploadedFile(fh, maxBytes)
			if err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
			data[fh.Filename] = content
		}
	}

	// subtasks 是前端编辑框里的子任务定义，直接当作 subtasks.json 参与校验与入库。
	if raw := form.Value["subtasks"]; len(raw) > 0 && strings.TrimSpace(raw[0]) != "" {
		data["subtasks.json"] = []byte(raw[0])
	}

	var scores []int
	if raw := form.Value["scores"]; len(raw) > 0 && strings.TrimSpace(raw[0]) != "" {
		if err := json.Unmarshal([]byte(raw[0]), &scores); err != nil {
			httpx.BadRequest(c, "分值格式错误")
			return
		}
	}

	if err := h.Testdata.Replace(p, data, scores); err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	items, err := h.testcaseItems(p.ID)
	if err != nil {
		httpx.ServerError(c, "查询测试数据失败")
		return
	}
	httpx.OK(c, gin.H{
		"items":       items,
		"total_score": p.FullScore(),
		"sync_failed": h.Testdata.SyncToNodes(syncContext(c), p.ID),
	})
}

// clearProblemTestdata 清空某题的测试数据。
func (h *Handler) clearProblemTestdata(c *gin.Context, p *model.Problem) {
	if err := h.Testdata.Delete(p.ID); err != nil {
		httpx.ServerError(c, "删除测试数据失败")
		return
	}
	httpx.OK(c, gin.H{"sync_failed": h.Testdata.ClearOnNodes(syncContext(c), p.ID)})
}

// respondProblemTestdata 返回某题的测试点清单。
func (h *Handler) respondProblemTestdata(c *gin.Context, p *model.Problem) {
	items, err := h.testcaseItems(p.ID)
	if err != nil {
		httpx.ServerError(c, "查询测试数据失败")
		return
	}
	httpx.OK(c, gin.H{"total_score": p.FullScore(), "items": items, "subtasks": p.Subtasks()})
}
