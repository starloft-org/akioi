package handler

import (
	"akioi/internal/httpx"

	"github.com/gin-gonic/gin"
)

// GlobalRanking 返回公开题库积分榜。
func (h *Handler) GlobalRanking(c *gin.Context) {
	page, pageSize, offset := httpx.Pagination(c)
	items, err := h.Rank.ComputeGlobal()
	if err != nil {
		httpx.ServerError(c, "查询排行榜失败")
		return
	}
	total := len(items)
	if offset > total {
		offset = total
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	httpx.OK(c, httpx.List(int64(total), page, pageSize, items[offset:end]))
}
