package handler

import (
	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

// AdminGetSettings 返回可热更新设置与环境密钥的配置状态，不返回密钥内容。
func (h *Handler) AdminGetSettings(c *gin.Context) {
	httpx.OK(c, gin.H{
		"settings": h.Settings.Current(),
		"secret_status": gin.H{
			"sms_credentials_configured":    h.Cfg.SMS.APIKey != "" && h.Cfg.SMS.APISecret != "",
			"wechat_credentials_configured": h.Cfg.WeChat.AppID != "" && h.Cfg.WeChat.AppSecret != "",
			"judge_sync_secret_configured":  h.Cfg.Judge.SyncSecret != "",
		},
	})
}

// AdminUpdateSettings 校验、持久化并立即切换运行时设置。
func (h *Handler) AdminUpdateSettings(c *gin.Context) {
	var req service.RuntimeSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	if req.SMSEnabled && (h.Cfg.SMS.APIKey == "" || h.Cfg.SMS.APISecret == "") {
		httpx.BadRequest(c, "启用短信服务前请先在 .env 配置 SMS_API_KEY 和 SMS_API_SECRET，并重启服务")
		return
	}
	if req.WeChatEnabled && (h.Cfg.WeChat.AppID == "" || h.Cfg.WeChat.AppSecret == "") {
		httpx.BadRequest(c, "启用微信登录前请先在 .env 配置 WECHAT_APPID 和 WECHAT_APPSECRET，并重启服务")
		return
	}
	if len(req.JudgeNodeEndpoints) > 0 && h.Cfg.Judge.SyncSecret == "" {
		httpx.BadRequest(c, "配置评测节点前请先在 .env 配置 JUDGE_SYNC_SECRET，并重启服务")
		return
	}
	settings, err := h.Settings.Update(req)
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	user := httpx.MustUser(c)
	if user == nil {
		return
	}
	logger.Adminf("管理员 %s 更新系统设置 ip=%s", user.Username, c.ClientIP())
	httpx.OK(c, gin.H{"settings": settings})
}
