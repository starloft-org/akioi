package handler

import (
	"errors"
	"net/http"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

// SiteCaptchaConfig 返回前端渲染人机验证所需的公开配置。
// 未开启时返回 enabled=false（前端跳过验证）；
// 已开启但验证码平台不可用时返回 503，让前端直接提示，而不是让用户提交后再失败。
func (h *Handler) SiteCaptchaConfig(c *gin.Context) {
	scene := c.DefaultQuery("scene", service.CaptchaSceneAccount)
	cfg, err := h.Captcha.Config(c.Request.Context(), scene)
	if err != nil {
		if errors.Is(err, service.ErrCaptchaNotEnabled) {
			httpx.OK(c, gin.H{"enabled": false})
			return
		}
		logger.Errorf("拉取验证码配置失败 scene=%s err=%v", scene, err)
		httpx.Fail(c, http.StatusServiceUnavailable, httpx.CodeServerError, "人机验证服务暂不可用，请稍后再试；若持续出现请联系管理员检查验证码配置")
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
