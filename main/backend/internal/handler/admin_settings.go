package handler

import (
	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

// adminSettingsRequest 把运行时设置与密钥增量放在同一个请求里提交，便于一次保存生效。
type adminSettingsRequest struct {
	service.RuntimeSettings
	Secrets service.SecretsPatch `json:"secrets"`
}

// AdminGetSettings 返回可热更新设置与密钥的配置状态，不返回密钥内容。
func (h *Handler) AdminGetSettings(c *gin.Context) {
	secrets := h.Settings.Secrets()
	httpx.OK(c, gin.H{
		"settings":      h.Settings.Current(),
		"secret_status": secrets.Status(),
		"secret_view":   secrets.View(),
	})
}

// AdminUpdateSettings 校验、持久化并立即切换运行时设置与密钥。
func (h *Handler) AdminUpdateSettings(c *gin.Context) {
	var req adminSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	settings, err := service.NormalizeRuntimeSettings(req.RuntimeSettings)
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	secrets, err := h.Settings.Secrets().Merge(req.Secrets)
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	if err := secrets.Validate(settings); err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	// 先保存密钥再切换设置：即使第二步失败，也只会保留未启用的旧设置，不会出现已启用但缺少密钥。
	secrets, err = h.Settings.UpdateSecrets(secrets)
	if err != nil {
		httpx.ServerError(c, err.Error())
		return
	}
	updated, err := h.Settings.Update(req.RuntimeSettings)
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	user := httpx.MustUser(c)
	if user == nil {
		return
	}
	logger.Adminf("管理员 %s 更新系统设置 ip=%s", user.Username, c.ClientIP())

	data := gin.H{
		"settings":      updated,
		"secret_status": secrets.Status(),
		"secret_view":   secrets.View(),
	}
	if req.Secrets.RegenerateJWT {
		data["notice"] = "登录密钥已重新生成，所有用户的登录状态已失效"
	}
	httpx.OK(c, data)
}
