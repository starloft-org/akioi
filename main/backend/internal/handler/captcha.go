package handler

import (
	"akioi/internal/httpx"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

// SiteCaptchaConfig 返回前端渲染人机验证所需的公开配置；未开启时 enabled=false。
func (h *Handler) SiteCaptchaConfig(c *gin.Context) {
	scene := c.DefaultQuery("scene", service.CaptchaSceneAccount)
	cfg, err := h.Captcha.Config(c.Request.Context(), scene)
	if err != nil {
		httpx.OK(c, gin.H{"enabled": false})
		return
	}
	httpx.OK(c, cfg)
}

// verifyCaptcha 校验前端取得的人机验证票据；后端未开启时直接放行。
func (h *Handler) verifyCaptcha(c *gin.Context, scene, ticket, randstr string) bool {
	if !h.Captcha.Enabled() {
		return true
	}
	if err := h.Captcha.Verify(c.Request.Context(), scene, ticket, randstr, c.ClientIP()); err != nil {
		httpx.BadRequest(c, err.Error())
		return false
	}
	return true
}
