package handler

import (
	"errors"
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
	Phone      string `json:"phone"`
	SMSCode    string `json:"sms_code"`
	Password   string `json:"password"`
	Nickname   string `json:"nickname"`
	InviteCode string `json:"invite_code"`
}

// Register 开放注册，受数据库注册开关与环境变量邀请码控制。
func (h *Handler) Register(c *gin.Context) {
	if !h.Settings.Current().RegisterEnabled || !h.SMS.Enabled() {
		httpx.Forbidden(c, "当前未开放注册")
		return
	}
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Phone = strings.TrimSpace(req.Phone)
	req.SMSCode = strings.TrimSpace(req.SMSCode)
	req.Nickname = strings.TrimSpace(req.Nickname)
	if err := validateUsername(req.Username); err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	if len(req.Password) < 6 || len(req.Password) > 64 {
		httpx.BadRequest(c, "密码长度需在 6 到 64 个字符之间")
		return
	}
	if !service.ValidPhone(req.Phone) {
		httpx.BadRequest(c, "请输入有效的中国大陆手机号")
		return
	}
	if h.Cfg.RegisterInviteCode != "" && req.InviteCode != h.Cfg.RegisterInviteCode {
		httpx.Forbidden(c, "邀请码不正确")
		return
	}

	var exists int64
	accounts := []string{req.Username, req.Phone}
	if err := h.DB.Model(&model.User{}).Where("username IN ? OR phone IN ?", accounts, accounts).Count(&exists).Error; err != nil {
		httpx.ServerError(c, "注册失败")
		return
	}
	if exists > 0 {
		httpx.Conflict(c, "用户名或手机号已被使用")
		return
	}
	if err := h.SMS.VerifyCode(c.Request.Context(), req.Phone, service.SMSPurposeRegister, req.SMSCode); err != nil {
		httpx.BadRequest(c, err.Error())
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
		Phone:        model.PhonePointer(req.Phone),
		PasswordHash: hash,
		Nickname:     req.Nickname,
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
	httpx.OK(c, gin.H{"token": token, "user": model.NewPrivateUser(&u)})
}

type loginRequest struct {
	Account  string `json:"account"`
	Username string `json:"username"` // 兼容旧客户端
	Password string `json:"password"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	account := strings.TrimSpace(req.Account)
	if account == "" {
		account = strings.TrimSpace(req.Username)
	}

	var u model.User
	if err := h.DB.Where("username = ? OR phone = ?", account, account).First(&u).Error; err != nil {
		httpx.Fail(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "账号或密码错误")
		return
	}
	if !h.Auth.CheckPassword(u.PasswordHash, req.Password) {
		logger.Businessf("登录失败 account=%s ip=%s", account, c.ClientIP())
		httpx.Fail(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "账号或密码错误")
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
	httpx.OK(c, gin.H{"token": token, "user": model.NewPrivateUser(&u)})
}

type sendSMSCodeRequest struct {
	Phone   string `json:"phone"`
	Purpose string `json:"purpose"`
}

// SendSMSCode 为注册或密码重置发送验证码，不提供短信登录用途。
func (h *Handler) SendSMSCode(c *gin.Context) {
	var req sendSMSCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if !service.ValidPhone(req.Phone) {
		httpx.BadRequest(c, "请输入有效的中国大陆手机号")
		return
	}
	if req.Purpose != service.SMSPurposeRegister && req.Purpose != service.SMSPurposeResetPassword {
		httpx.BadRequest(c, "验证码用途不正确")
		return
	}
	settings := h.Settings.Current()
	if req.Purpose == service.SMSPurposeRegister && !settings.RegisterEnabled {
		httpx.Forbidden(c, "当前未开放注册")
		return
	}
	var count int64
	if err := h.DB.Model(&model.User{}).Where("phone = ?", req.Phone).Count(&count).Error; err != nil {
		httpx.ServerError(c, "发送验证码失败")
		return
	}
	if req.Purpose == service.SMSPurposeRegister && count > 0 {
		httpx.Conflict(c, "手机号已被使用")
		return
	}
	if req.Purpose == service.SMSPurposeResetPassword && count == 0 {
		// 保持与成功请求一致的响应，避免枚举已注册手机号。
		httpx.OK(c, gin.H{"expires_in": settings.SMSCodeTTLMinutes * 60})
		return
	}
	if err := h.SMS.SendCode(c.Request.Context(), req.Phone, req.Purpose); err != nil {
		if errors.Is(err, service.ErrSMSTooFrequent) {
			httpx.TooMany(c, httpx.CodeTooManyRequest, err.Error())
			return
		}
		logger.Errorf("短信验证码发送失败 purpose=%s phone=%s err=%v", req.Purpose, maskPhone(req.Phone), err)
		httpx.ServerError(c, "验证码发送失败")
		return
	}
	httpx.OK(c, gin.H{"expires_in": settings.SMSCodeTTLMinutes * 60})
}

type resetPasswordRequest struct {
	Phone       string `json:"phone"`
	SMSCode     string `json:"sms_code"`
	NewPassword string `json:"new_password"`
}

// ResetPassword 仅允许通过已绑定手机号的短信验证码重置密码。
func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	req.SMSCode = strings.TrimSpace(req.SMSCode)
	if !service.ValidPhone(req.Phone) {
		httpx.BadRequest(c, "请输入有效的中国大陆手机号")
		return
	}
	if len(req.NewPassword) < 6 || len(req.NewPassword) > 64 {
		httpx.BadRequest(c, "密码长度需在 6 到 64 个字符之间")
		return
	}
	var u model.User
	if err := h.DB.Where("phone = ?", req.Phone).First(&u).Error; err != nil {
		httpx.BadRequest(c, "验证码错误或已过期")
		return
	}
	if err := h.SMS.VerifyCode(c.Request.Context(), req.Phone, service.SMSPurposeResetPassword, req.SMSCode); err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	if err := h.Auth.ResetPassword(u.ID, req.NewPassword); err != nil {
		httpx.ServerError(c, "重置密码失败")
		return
	}
	logger.Businessf("用户通过短信重置密码 username=%s ip=%s", u.Username, c.ClientIP())
	httpx.OK(c, gin.H{})
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
	httpx.OK(c, gin.H{"user": model.NewPrivateUser(u)})
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
	httpx.OK(c, gin.H{"token": token, "user": model.NewPrivateUser(u), "redirect": redirect})
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

func maskPhone(phone string) string {
	if len(phone) != 11 {
		return "***"
	}
	return phone[:3] + "****" + phone[7:]
}
