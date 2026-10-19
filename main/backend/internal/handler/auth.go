package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

type registerRequest struct {
	Username string `json:"username"`
	Channel  string `json:"channel"` // sms | email，缺省按 sms 处理
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	SMSCode  string `json:"sms_code"`
	Code     string `json:"code"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

// Register 开放注册：短信与邮件任一渠道开放即可注册，都未开放时不支持注册。
func (h *Handler) Register(c *gin.Context) {
	settings := h.Settings.Current()
	smsOK := h.SMS.Enabled()
	emailOK := h.Email.Enabled()
	if !settings.RegisterEnabled {
		httpx.Forbidden(c, "当前未开放注册")
		return
	}
	if !smsOK && !emailOK {
		httpx.BadRequest(c, "注册渠道未开放，暂时无法注册")
		return
	}
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Channel = strings.ToLower(strings.TrimSpace(req.Channel))
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.TrimSpace(req.Email)
	req.SMSCode = strings.TrimSpace(req.SMSCode)
	req.Code = strings.TrimSpace(req.Code)
	req.Nickname = strings.TrimSpace(req.Nickname)
	if req.Channel == "" {
		req.Channel = "sms"
	}
	if err := validateUsername(req.Username); err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	if len(req.Password) < 6 || len(req.Password) > 64 {
		httpx.BadRequest(c, "密码长度需在 6 到 64 个字符之间")
		return
	}

	var verifyTarget, purpose string
	switch req.Channel {
	case "email":
		if !emailOK {
			httpx.BadRequest(c, "邮件注册未开放，请改用短信注册")
			return
		}
		if !service.ValidEmail(req.Email) {
			httpx.BadRequest(c, "请输入有效的邮箱地址")
			return
		}
		verifyTarget, purpose = req.Email, service.EmailPurposeRegister
	case "sms":
		if !smsOK {
			httpx.BadRequest(c, "短信注册未开放，请改用邮件注册")
			return
		}
		if !service.ValidPhone(req.Phone) {
			httpx.BadRequest(c, "请输入有效的中国大陆手机号")
			return
		}
		verifyTarget, purpose = req.Phone, service.SMSPurposeRegister
	default:
		httpx.BadRequest(c, "注册渠道无效")
		return
	}

	var exists int64
	var dupErr error
	if req.Channel == "email" {
		dupErr = h.DB.Model(&model.User{}).
			Where("username = ? OR email = ?", req.Username, req.Email).
			Count(&exists).Error
	} else {
		accounts := []string{req.Username, req.Phone}
		dupErr = h.DB.Model(&model.User{}).
			Where("username IN ? OR phone IN ?", accounts, accounts).
			Count(&exists).Error
	}
	if dupErr != nil {
		httpx.ServerError(c, "注册失败")
		return
	}
	if exists > 0 {
		httpx.Conflict(c, "用户名或联系方式已被使用")
		return
	}

	var verifyErr error
	if req.Channel == "email" {
		verifyErr = h.Email.VerifyCode(c.Request.Context(), verifyTarget, purpose, req.Code)
	} else {
		verifyErr = h.SMS.VerifyCode(c.Request.Context(), verifyTarget, purpose, req.SMSCode)
	}
	if verifyErr != nil {
		httpx.BadRequest(c, verifyErr.Error())
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
		Role:         model.RoleUser,
		Status:       model.UserStatusActive,
	}
	if req.Channel == "email" {
		u.Email = &req.Email
	} else {
		u.Phone = model.PhonePointer(req.Phone)
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
		if errors.Is(err, service.ErrSMSNotConfigured) {
			httpx.BadRequest(c, "短信服务未配置，暂时无法发送验证码")
			return
		}
		logger.Errorf("短信验证码发送失败 purpose=%s phone=%s err=%v", req.Purpose, maskPhone(req.Phone), err)
		httpx.ServerError(c, "验证码发送失败")
		return
	}
	httpx.OK(c, gin.H{"expires_in": settings.SMSCodeTTLMinutes * 60})
}

type sendEmailCodeRequest struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"`
}

// SendEmailCode 为注册或密码重置发送邮件验证码。
func (h *Handler) SendEmailCode(c *gin.Context) {
	var req sendEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if !service.ValidEmail(req.Email) {
		httpx.BadRequest(c, "请输入有效的邮箱地址")
		return
	}
	if req.Purpose != service.EmailPurposeRegister && req.Purpose != service.EmailPurposeResetPassword {
		httpx.BadRequest(c, "验证码用途不正确")
		return
	}
	settings := h.Settings.Current()
	if req.Purpose == service.EmailPurposeRegister && !settings.RegisterEnabled {
		httpx.Forbidden(c, "当前未开放注册")
		return
	}
	var count int64
	if err := h.DB.Model(&model.User{}).Where("email = ?", req.Email).Count(&count).Error; err != nil {
		httpx.ServerError(c, "发送验证码失败")
		return
	}
	if req.Purpose == service.EmailPurposeRegister && count > 0 {
		httpx.Conflict(c, "邮箱已被使用")
		return
	}
	if req.Purpose == service.EmailPurposeResetPassword && count == 0 {
		// 保持与成功请求一致的响应，避免枚举已注册邮箱。
		httpx.OK(c, gin.H{"expires_in": settings.SMSCodeTTLMinutes * 60})
		return
	}
	if err := h.Email.SendCode(c.Request.Context(), req.Email, req.Purpose); err != nil {
		if errors.Is(err, service.ErrEmailTooFrequent) {
			httpx.TooMany(c, httpx.CodeTooManyRequest, err.Error())
			return
		}
		if errors.Is(err, service.ErrEmailNotConfigured) {
			httpx.BadRequest(c, "邮件服务未配置，暂时无法发送验证码")
			return
		}
		logger.Errorf("邮件验证码发送失败 purpose=%s err=%v", req.Purpose, err)
		httpx.ServerError(c, "验证码发送失败")
		return
	}
	httpx.OK(c, gin.H{"expires_in": settings.SMSCodeTTLMinutes * 60})
}

type resetPasswordRequest struct {
	Channel     string `json:"channel"` // sms | email，缺省按 sms 处理
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	SMSCode     string `json:"sms_code"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

// ResetPassword 支持短信或邮件渠道重置密码。
func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	req.Channel = strings.ToLower(strings.TrimSpace(req.Channel))
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.TrimSpace(req.Email)
	req.SMSCode = strings.TrimSpace(req.SMSCode)
	req.Code = strings.TrimSpace(req.Code)
	if req.Channel == "" {
		req.Channel = "sms"
	}
	if len(req.NewPassword) < 6 || len(req.NewPassword) > 64 {
		httpx.BadRequest(c, "密码长度需在 6 到 64 个字符之间")
		return
	}

	var u model.User
	switch req.Channel {
	case "email":
		if !h.Email.Enabled() {
			httpx.BadRequest(c, "邮件服务未配置，暂时无法重置密码")
			return
		}
		if !service.ValidEmail(req.Email) {
			httpx.BadRequest(c, "请输入有效的邮箱地址")
			return
		}
		if err := h.DB.Where("email = ?", req.Email).First(&u).Error; err != nil {
			httpx.BadRequest(c, "验证码错误或已过期")
			return
		}
		if err := h.Email.VerifyCode(c.Request.Context(), req.Email, service.EmailPurposeResetPassword, req.Code); err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
	case "sms":
		if !service.ValidPhone(req.Phone) {
			httpx.BadRequest(c, "请输入有效的中国大陆手机号")
			return
		}
		if err := h.DB.Where("phone = ?", req.Phone).First(&u).Error; err != nil {
			httpx.BadRequest(c, "验证码错误或已过期")
			return
		}
		if err := h.SMS.VerifyCode(c.Request.Context(), req.Phone, service.SMSPurposeResetPassword, req.SMSCode); err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
	default:
		httpx.BadRequest(c, "重置渠道无效")
		return
	}
	if err := h.Auth.ResetPassword(u.ID, req.NewPassword); err != nil {
		httpx.ServerError(c, "重置密码失败")
		return
	}
	logger.Businessf("用户重置密码成功 username=%s channel=%s ip=%s", u.Username, req.Channel, c.ClientIP())
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
	httpx.OK(c, gin.H{
		"user":         model.NewPrivateUser(u),
		"wechat_bound": h.WeChat.Bound(u.ID),
	})
}

// WeChatURL 返回微信扫码地址；purpose=bind 时生成绑定用的地址，需要已登录。
func (h *Handler) WeChatURL(c *gin.Context) {
	if !h.WeChat.Enabled() {
		httpx.OK(c, gin.H{"enabled": false, "url": ""})
		return
	}
	st := service.WeChatState{Purpose: "login", Redirect: c.Query("redirect"), From: c.DefaultQuery("from", "h5")}
	if c.Query("purpose") == "bind" {
		u := httpx.CurrentUser(c)
		if u == nil {
			httpx.Unauthorized(c, "请先登录")
			return
		}
		st.Purpose = "bind"
		st.UserID = u.ID
	}
	state, err := randomState()
	if err != nil {
		httpx.ServerError(c, "生成登录状态失败")
		return
	}
	if err := h.WeChat.SaveState(c.Request.Context(), state, st); err != nil {
		httpx.ServerError(c, "生成登录状态失败")
		return
	}
	httpx.OK(c, gin.H{"enabled": true, "url": h.WeChat.AuthorizeURL(state), "state": state})
}

// WeChatStatus 供 PC 端轮询扫码结果：pending 表示还没扫或授权未完成。
func (h *Handler) WeChatStatus(c *gin.Context) {
	state := c.Query("state")
	if state == "" {
		httpx.BadRequest(c, "缺少 state")
		return
	}
	res, ok := h.WeChat.GetScanResult(c.Request.Context(), state)
	if !ok {
		httpx.OK(c, gin.H{"status": "pending"})
		return
	}
	if res.Bound {
		httpx.OK(c, gin.H{"status": "bound", "redirect": res.Redirect})
		return
	}
	var u model.User
	if err := h.DB.First(&u, res.UserID).Error; err != nil {
		httpx.OK(c, gin.H{"status": "pending"})
		return
	}
	httpx.OK(c, gin.H{
		"status":   "authorized",
		"token":    res.Token,
		"user":     model.NewPrivateUser(&u),
		"redirect": res.Redirect,
	})
}

// WeChatUnbind 解除当前账号的微信绑定。
func (h *Handler) WeChatUnbind(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	if err := h.WeChat.Unbind(u.ID); err != nil {
		if errors.Is(err, service.ErrWechatNotBound) {
			httpx.BadRequest(c, err.Error())
			return
		}
		logger.Errorf("解除微信绑定失败 user=%d: %v", u.ID, err)
		httpx.ServerError(c, "解除绑定失败")
		return
	}
	logger.Businessf("用户 %d 解除微信绑定", u.ID)
	httpx.OK(c, gin.H{"bound": false})
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
	st, ok := h.WeChat.ConsumeState(c.Request.Context(), c.Query("state"))
	if !ok {
		httpx.BadRequest(c, "登录状态已失效，请重新发起")
		return
	}
	profile, err := h.WeChat.Exchange(c.Request.Context(), code)
	if err != nil {
		logger.Errorf("微信授权换取资料失败: %v", err)
		httpx.ServerError(c, "微信授权失败")
		return
	}
	// 已登录用户发起的是绑定流程，不再创建新账号。
	if st.Purpose == "bind" {
		if err := h.WeChat.BindTo(st.UserID, profile); err != nil {
			if errors.Is(err, service.ErrWechatBoundToOther) || errors.Is(err, service.ErrWechatAlreadyBound) {
				httpx.Conflict(c, err.Error())
				return
			}
			logger.Errorf("微信绑定失败 user=%d: %v", st.UserID, err)
			httpx.ServerError(c, "微信绑定失败")
			return
		}
		logger.Businessf("用户 %d 绑定微信成功", st.UserID)
		_ = h.WeChat.SetScanResult(c.Request.Context(), c.Query("state"), service.ScanResult{
			Bound:    true,
			UserID:   st.UserID,
			Redirect: st.Redirect,
		})
		httpx.OK(c, gin.H{"bound": true, "redirect": st.Redirect, "from": st.From})
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
	// 结果同时暂存，供 PC 端的二维码页面轮询取走。
	_ = h.WeChat.SetScanResult(c.Request.Context(), c.Query("state"), service.ScanResult{
		Token:    token,
		UserID:   u.ID,
		Redirect: st.Redirect,
	})
	httpx.OK(c, gin.H{"token": token, "user": model.NewPrivateUser(u), "redirect": st.Redirect, "from": st.From})
}

func bearerOf(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return strings.TrimSpace(h)
}

func validateUsername(name string) error {
	// 先校验字符合法性：全角字母、中文等会被拦在这里，
	// 否则多字节字符会让字节长度超标，报出误导性的"长度"错误。
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return errString("用户名只能包含字母、数字、下划线与短横线")
		}
	}
	// 按 Unicode 字符数而非字节数统计，与提示文案的"字符"语义一致
	n := utf8.RuneCountInString(name)
	if n < 4 || n > 20 {
		return errString("用户名长度需在 4 到 20 个字符之间")
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
