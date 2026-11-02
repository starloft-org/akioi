package handler

import (
	"akioi/internal/httpx"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
)

// ListMyTeams 返回当前用户加入的团队与团队身份，用于选择“导入到团队”的目标。
func (h *Handler) ListMyTeams(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var members []model.TeamMember
	if err := h.DB.Where("user_id = ?", u.ID).Find(&members).Error; err != nil {
		httpx.ServerError(c, "查询团队失败")
		return
	}
	roleOf := make(map[uint]string, len(members))
	teamIDs := make([]uint, 0, len(members))
	for i := range members {
		roleOf[members[i].TeamID] = members[i].Role
		teamIDs = append(teamIDs, members[i].TeamID)
	}
	var teams []model.Team
	if len(teamIDs) > 0 {
		if err := h.DB.Where("id IN ?", teamIDs).Order("id asc").Find(&teams).Error; err != nil {
			httpx.ServerError(c, "查询团队失败")
			return
		}
	}
	items := make([]gin.H, 0, len(teams))
	for i := range teams {
		items = append(items, gin.H{
			"id":           teams[i].ID,
			"name":         teams[i].Name,
			"member_role":  roleOf[teams[i].ID],
			"member_count": teams[i].MemberCount,
		})
	}
	httpx.OK(c, gin.H{"items": items})
}
