package handler

import (
	"net/http"
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

type registerRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Nickname   string `json:"nickname"`
	Email      string `json:"email"`
	InviteCode string `json:"invite_code"`
}

// Register 开放注册，受 REGISTER_ENABLED 与邀请码控制。
func (h *Handler) Register(c *gin.Context) {
	if !h.Cfg.RegisterEnabled {
		httpx.Forbidden(c, "当前未开放注册")
		return
	}
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	if err := validateUsername(req.Username); err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	if len(req.Password) < 6 || len(req.Password) > 64 {
		httpx.BadRequest(c, "密码长度需在 6 到 64 个字符之间")
		return
	}
	if h.Cfg.RegisterInviteCode != "" && req.InviteCode != h.Cfg.RegisterInviteCode {
		httpx.Forbidden(c, "邀请码不正确")
		return
	}

	var exists int64
	if err := h.DB.Model(&model.User{}).Where("username = ?", req.Username).Count(&exists).Error; err != nil {
		httpx.ServerError(c, "注册失败")
		return
	}
	if exists > 0 {
		httpx.Conflict(c, "用户名已被占用")
		return
	}

	hash, err := h.Auth.HashPassword(req.Password)
	if err != nil {
		httpx.ServerError(c, "注册失败")
		return
	}
	if req.Nickname == "" {
		req.Nickname = req.Username
	}
	u := model.User{
		Username:     req.Username,
		PasswordHash: hash,
		Nickname:     req.Nickname,
		Email:        strings.TrimSpace(req.Email),
		Role:         model.RoleUser,
		Status:       model.UserStatusActive,
	}
	if err := h.DB.Create(&u).Error; err != nil {
		httpx.ServerError(c, "注册失败")
		return
	}
	token, err := h.Auth.Issue(&u)
	if err != nil {
		httpx.ServerError(c, "登录凭证签发失败")
		return
	}
	logger.Businessf("新用户注册成功 username=%s ip=%s", u.Username, c.ClientIP())
	httpx.OK(c, gin.H{"token": token, "user": model.NewPublicUser(&u)})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)

	var u model.User
	if err := h.DB.Where("username = ?", req.Username).First(&u).Error; err != nil {
		httpx.Fail(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "用户名或密码错误")
		return
	}
	if !h.Auth.CheckPassword(u.PasswordHash, req.Password) {
		logger.Businessf("登录失败 username=%s ip=%s", req.Username, c.ClientIP())
		httpx.Fail(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "用户名或密码错误")
		return
	}
	if u.Status != model.UserStatusActive {
		httpx.Forbidden(c, service.ErrUserBanned.Error())
		return
	}
	token, err := h.Auth.Issue(&u)
	if err != nil {
		httpx.ServerError(c, "登录凭证签发失败")
		return
	}
	now := time.Now()
	h.DB.Model(&model.User{}).Where("id = ?", u.ID).UpdateColumn("last_login_at", now)
	u.LastLoginAt = &now
	logger.Businessf("登录成功 username=%s ip=%s", u.Username, c.ClientIP())
	httpx.OK(c, gin.H{"token": token, "user": model.NewPublicUser(&u)})
}

func (h *Handler) Logout(c *gin.Context) {
	if err := h.Auth.Revoke(bearerOf(c)); err != nil {
		logger.Errorf("登出失败: %v", err)
	}
	httpx.OK(c, gin.H{})
}

func (h *Handler) Me(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	httpx.OK(c, gin.H{"user": model.NewPublicUser(u)})
}

// WeChatURL 返回微信扫码登录地址；未启用时 enabled 为 false。
func (h *Handler) WeChatURL(c *gin.Context) {
	if !h.WeChat.Enabled() {
		httpx.OK(c, gin.H{"enabled": false, "url": ""})
		return
	}
	state, err := randomState()
	if err != nil {
		httpx.ServerError(c, "生成登录状态失败")
		return
	}
	if err := h.WeChat.SaveState(c.Request.Context(), state, c.Query("redirect")); err != nil {
		httpx.ServerError(c, "生成登录状态失败")
		return
	}
	httpx.OK(c, gin.H{"enabled": true, "url": h.WeChat.AuthorizeURL(state), "state": state})
}

// WeChatCallback 处理微信回调：换取资料、绑定或创建账号并签发登录凭证。
func (h *Handler) WeChatCallback(c *gin.Context) {
	if !h.WeChat.Enabled() {
		httpx.Forbidden(c, "微信登录未启用")
		return
	}
	code := c.Query("code")
	if code == "" {
		httpx.BadRequest(c, "缺少授权码")
		return
	}
	redirect, ok := h.WeChat.ConsumeState(c.Request.Context(), c.Query("state"))
	if !ok {
		httpx.BadRequest(c, "登录状态已失效，请重新发起")
		return
	}
	profile, err := h.WeChat.Exchange(c.Request.Context(), code)
	if err != nil {
		logger.Errorf("微信登录换取资料失败: %v", err)
		httpx.ServerError(c, "微信登录失败")
		return
	}
	u, err := h.WeChat.BindOrCreate(profile)
	if err != nil {
		logger.Errorf("微信登录绑定失败: %v", err)
		httpx.ServerError(c, "微信登录失败")
		return
	}
	if u.Status != model.UserStatusActive {
		httpx.Forbidden(c, service.ErrUserBanned.Error())
		return
	}
	token, err := h.Auth.Issue(u)
	if err != nil {
		httpx.ServerError(c, "登录凭证签发失败")
		return
	}
	now := time.Now()
	h.DB.Model(&model.User{}).Where("id = ?", u.ID).UpdateColumn("last_login_at", now)
	logger.Businessf("微信登录成功 username=%s ip=%s", u.Username, c.ClientIP())
	httpx.OK(c, gin.H{"token": token, "user": model.NewPublicUser(u), "redirect": redirect})
}

func bearerOf(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return strings.TrimSpace(h)
}

func validateUsername(name string) error {
	if len(name) < 3 || len(name) > 20 {
		return errString("用户名长度需在 3 到 20 个字符之间")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return errString("用户名只能包含字母、数字、下划线与短横线")
		}
	}
	return nil
}

func randomState() (string, error) {
	return service.NewStateToken()
}

type errString string

func (e errString) Error() string { return string(e) }
