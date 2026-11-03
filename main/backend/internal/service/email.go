package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"akioi/internal/config"
	rediskeys "akioi/internal/redis"

	goredis "github.com/redis/go-redis/v9"
)

const (
	EmailPurposeRegister      = "register"
	EmailPurposeResetPassword = "reset_password"
)

var (
	ErrEmailNotConfigured = errors.New("邮件服务尚未配置")
	ErrEmailInvalid       = errors.New("邮箱格式不正确")
	// 复用短信验证码的错误文案，前端提示保持一致。
	ErrEmailTooFrequent = ErrSMSTooFrequent
	ErrEmailCodeInvalid = ErrSMSCodeInvalid
)

var emailPattern = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)

func ValidEmail(email string) bool {
	return len(email) <= 255 && emailPattern.MatchString(email)
}

func validEmailPurpose(purpose string) bool {
	return purpose == EmailPurposeRegister || purpose == EmailPurposeResetPassword
}

// EmailService 通过 SMTP 发送注册与密码重置验证码，验证码的存储与校验
// 复用短信的 Redis 结构，仅把标识符从手机号换成邮箱。
type EmailService struct {
	rdb      *goredis.Client
	cfg      *config.Config
	settings *SettingService
}

func NewEmail(rdb *goredis.Client, cfg *config.Config, settings *SettingService) *EmailService {
	return &EmailService{rdb: rdb, cfg: cfg, settings: settings}
}

// Enabled 需要后台开关与后台填写的 SMTP 配置同时就绪。
func (s *EmailService) Enabled() bool {
	secrets := s.settings.Secrets()
	return s.settings.Current().EmailEnabled && secrets.SMTPHost != "" && secrets.SMTPFrom != ""
}

// smtpConfig 把数据库中的邮件配置转换为发送函数需要的结构。
func (s *EmailService) smtpConfig() config.SMTPConfig {
	secrets := s.settings.Secrets()
	return config.SMTPConfig{
		Host:     secrets.SMTPHost,
		Port:     secrets.SMTPPort,
		Username: secrets.SMTPUsername,
		Password: secrets.SMTPPassword,
		From:     secrets.SMTPFrom,
	}
}

// SendCode 生成一次性验证码，通过邮件发送并写入 Redis。
func (s *EmailService) SendCode(ctx context.Context, email, purpose string) error {
	if !s.Enabled() {
		return ErrEmailNotConfigured
	}
	if !ValidEmail(email) || !validEmailPurpose(purpose) {
		return ErrEmailInvalid
	}

	settings := s.settings.Current()
	cooldownKey := rediskeys.KeySMSCooldownPrefix + purpose + ":" + email
	ok, err := s.rdb.SetNX(ctx, cooldownKey, "1", time.Duration(settings.SMSSendIntervalSec)*time.Second).Result()
	if err != nil {
		return fmt.Errorf("写入限流失败: %w", err)
	}
	if !ok {
		return ErrEmailTooFrequent
	}

	code, err := randomDigits(6)
	if err != nil {
		_ = s.rdb.Del(ctx, cooldownKey).Err()
		return err
	}
	ttl := time.Duration(settings.SMSCodeTTLMinutes) * time.Minute
	codeKey := rediskeys.KeySMSCodePrefix + purpose + ":" + email
	attemptsKey := rediskeys.KeySMSAttemptsPrefix + purpose + ":" + email
	if err := s.rdb.Set(ctx, codeKey, s.codeDigest(email, purpose, code), ttl).Err(); err != nil {
		_ = s.rdb.Del(ctx, cooldownKey).Err()
		return fmt.Errorf("保存验证码失败: %w", err)
	}
	_ = s.rdb.Del(ctx, attemptsKey).Err()

	if err := s.send(ctx, email, purpose, code); err != nil {
		_ = s.rdb.Del(ctx, codeKey, cooldownKey).Err()
		return err
	}
	return nil
}

// VerifyCode 校验并消费验证码，同一验证码最多允许尝试五次。
func (s *EmailService) VerifyCode(ctx context.Context, email, purpose, code string) error {
	if !ValidEmail(email) || !validEmailPurpose(purpose) || len(code) != 6 {
		return ErrEmailCodeInvalid
	}
	codeKey := rediskeys.KeySMSCodePrefix + purpose + ":" + email
	attemptsKey := rediskeys.KeySMSAttemptsPrefix + purpose + ":" + email
	stored, err := s.rdb.Get(ctx, codeKey).Result()
	if err != nil {
		return ErrEmailCodeInvalid
	}
	if !hmac.Equal([]byte(stored), []byte(s.codeDigest(email, purpose, code))) {
		attempts, _ := s.rdb.Incr(ctx, attemptsKey).Result()
		_ = s.rdb.Expire(ctx, attemptsKey, time.Duration(s.settings.Current().SMSCodeTTLMinutes)*time.Minute).Err()
		if attempts >= maxSMSVerifyAttempts {
			_ = s.rdb.Del(ctx, codeKey, attemptsKey).Err()
		}
		return ErrEmailCodeInvalid
	}
	_ = s.rdb.Del(ctx, codeKey, attemptsKey).Err()
	return nil
}

func (s *EmailService) codeDigest(email, purpose, code string) string {
	mac := hmac.New(sha256.New, []byte(s.settings.Secrets().JWTSecret))
	_, _ = mac.Write([]byte(email + "\n" + purpose + "\n" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *EmailService) send(ctx context.Context, to, purpose, code string) error {
	settings := s.settings.Current()
	label := "注册验证码"
	if purpose == EmailPurposeResetPassword {
		label = "密码重置验证码"
	}
	subject := fmt.Sprintf("%s%s", settings.AppName, label)
	body := buildCodeEmailHTML(settings.AppName, label, code, settings.SMSCodeTTLMinutes)
	return sendMail(s.smtpConfig(), to, subject, body)
}

func buildCodeEmailHTML(appName, label, code string, ttlMinutes int) string {
	return fmt.Sprintf(`<!doctype html><html><body style="margin:0;padding:24px;background:#f5f7fa;font-family:'PingFang SC','Microsoft YaHei',sans-serif;">
<div style="max-width:480px;margin:0 auto;background:#fff;border-radius:8px;padding:32px;">
<h2 style="margin:0 0 16px;color:#1d2129;font-size:18px;">%s %s</h2>
<p style="margin:0 0 12px;color:#4e5969;font-size:14px;line-height:1.8;">你好！你正在使用邮箱%s，本次验证码为：</p>
<p style="margin:0 0 12px;font-size:28px;font-weight:700;letter-spacing:8px;color:#1d2129;">%s</p>
<p style="margin:0;color:#86909c;font-size:13px;line-height:1.8;">验证码 %d 分钟内有效。若非本人操作，请忽略本邮件。</p>
</div></body></html>`, appName, label, mapPurposeVerb(label), code, ttlMinutes)
}

func mapPurposeVerb(label string) string {
	if strings.Contains(label, "重置") {
		return "重置密码"
	}
	return "注册"
}

// loginAuth 实现 SMTP LOGIN 认证（QQ、163 等国内服务商普遍只支持该方式）。
type loginAuth struct {
	username string
	password string
}

func (a loginAuth) Start(info *smtp.ServerInfo) (string, []byte, error) {
	// 认证凭据只能在加密连接上发送；明文连接直接拒绝，不做降级。
	if !info.TLS {
		return "", nil, fmt.Errorf("拒绝在未加密的 SMTP 连接上发送认证凭据")
	}
	return "LOGIN", []byte(a.username), nil
}

func (a loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(string(fromServer))) {
	case "username:":
		return []byte(a.username), nil
	case "password:":
		return []byte(a.password), nil
	}
	return nil, fmt.Errorf("未知的 SMTP 认证步骤")
}

// sendMail 通过 SMTP 发送 HTML 邮件：465 端口使用隐式 TLS，其余端口尝试 STARTTLS。
func sendMail(cfg config.SMTPConfig, to, subject, htmlBody string) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	subjectHeader := "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?="
	message := strings.Join([]string{
		"From: " + cfg.From,
		"To: " + to,
		"Subject: " + subjectHeader,
		"MIME-Version: 1.0",
		`Content-Type: text/html; charset="UTF-8"`,
		"",
		htmlBody,
	}, "\r\n")

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = loginAuth{username: cfg.Username, password: cfg.Password}
	}

	if cfg.Port == 465 {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr,
			&tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return fmt.Errorf("连接 SMTP 服务器失败: %w", err)
		}
		return deliverOverTLS(conn, cfg.Host, auth, cfg.From, to, message)
	}
	return deliverWithSTARTTLS(addr, cfg.Host, auth, cfg.From, to, message)
}

func deliverOverTLS(conn *tls.Conn, host string, auth smtp.Auth, from, to, message string) error {
	defer func() { _ = conn.Close() }()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("建立 SMTP 会话失败: %w", err)
	}
	return deliver(client, auth, from, to, message)
}

func deliverWithSTARTTLS(addr, host string, auth smtp.Auth, from, to, message string) error {
	client, err := dialSMTP(addr, host)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("SMTP 服务器不支持 STARTTLS，已拒绝在不加密的连接上发送邮件")
	}
	if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("STARTTLS 升级失败: %w", err)
	}
	return deliver(client, auth, from, to, message)
}

// dialSMTP 带超时连接 SMTP 服务器，避免网络异常时长期占用协程。
func dialSMTP(addr, host string) (*smtp.Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("连接 SMTP 服务器失败: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("建立 SMTP 会话失败: %w", err)
	}
	return client, nil
}

func deliver(client *smtp.Client, auth smtp.Auth, from, to, message string) error {
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP 认证失败: %w", err)
			}
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("设置收件人失败: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(message)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
