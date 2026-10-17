package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"akioi/internal/config"
	rediskeys "akioi/internal/redis"

	goredis "github.com/redis/go-redis/v9"
)

const (
	SMSPurposeRegister      = "register"
	SMSPurposeResetPassword = "reset_password"
	maxSMSVerifyAttempts    = 5
)

var (
	ErrSMSNotConfigured = errors.New("短信服务尚未配置")
	ErrSMSTooFrequent   = errors.New("验证码发送过于频繁，请稍后再试")
	ErrSMSCodeInvalid   = errors.New("验证码错误或已过期")
)

type SMSService struct {
	rdb    *goredis.Client
	cfg    *config.Config
	client *http.Client
}

func NewSMS(rdb *goredis.Client, cfg *config.Config) *SMSService {
	return &SMSService{
		rdb: rdb,
		cfg: cfg,
		client: &http.Client{Timeout: time.Duration(cfg.SMS.RequestTimeoutSec) * time.Second},
	}
}

func (s *SMSService) Enabled() bool { return s.cfg.SMS.Enabled() }

func ValidPhone(phone string) bool {
	if len(phone) != 11 || phone[0] != '1' || phone[1] < '3' || phone[1] > '9' {
		return false
	}
	for _, r := range phone {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// SendCode 生成一次性验证码，通过 StarLoft 短信 API 发送并写入 Redis。
func (s *SMSService) SendCode(ctx context.Context, phone, purpose string) error {
	if !s.Enabled() {
		return ErrSMSNotConfigured
	}
	if !ValidPhone(phone) || !validSMSPurpose(purpose) {
		return ErrSMSCodeInvalid
	}

	cooldownKey := rediskeys.KeySMSCooldownPrefix + purpose + ":" + phone
	ok, err := s.rdb.SetNX(ctx, cooldownKey, "1", time.Duration(s.cfg.SMS.SendIntervalSec)*time.Second).Result()
	if err != nil {
		return fmt.Errorf("写入短信限流失败: %w", err)
	}
	if !ok {
		return ErrSMSTooFrequent
	}

	code, err := randomDigits(6)
	if err != nil {
		_ = s.rdb.Del(ctx, cooldownKey).Err()
		return err
	}
	ttl := time.Duration(s.cfg.SMS.CodeTTLMinutes) * time.Minute
	codeKey := rediskeys.KeySMSCodePrefix + purpose + ":" + phone
	attemptsKey := rediskeys.KeySMSAttemptsPrefix + purpose + ":" + phone
	if err := s.rdb.Set(ctx, codeKey, s.codeDigest(phone, purpose, code), ttl).Err(); err != nil {
		_ = s.rdb.Del(ctx, cooldownKey).Err()
		return fmt.Errorf("保存验证码失败: %w", err)
	}
	_ = s.rdb.Del(ctx, attemptsKey).Err()

	if err := s.send(ctx, phone, purpose, code); err != nil {
		_ = s.rdb.Del(ctx, codeKey, cooldownKey).Err()
		return err
	}
	return nil
}

// VerifyCode 校验并消费验证码，同一验证码最多允许尝试五次。
func (s *SMSService) VerifyCode(ctx context.Context, phone, purpose, code string) error {
	if !ValidPhone(phone) || !validSMSPurpose(purpose) || len(code) != 6 {
		return ErrSMSCodeInvalid
	}
	codeKey := rediskeys.KeySMSCodePrefix + purpose + ":" + phone
	attemptsKey := rediskeys.KeySMSAttemptsPrefix + purpose + ":" + phone
	stored, err := s.rdb.Get(ctx, codeKey).Result()
	if err != nil {
		return ErrSMSCodeInvalid
	}
	if !hmac.Equal([]byte(stored), []byte(s.codeDigest(phone, purpose, code))) {
		attempts, _ := s.rdb.Incr(ctx, attemptsKey).Result()
		_ = s.rdb.Expire(ctx, attemptsKey, time.Duration(s.cfg.SMS.CodeTTLMinutes)*time.Minute).Err()
		if attempts >= maxSMSVerifyAttempts {
			_ = s.rdb.Del(ctx, codeKey, attemptsKey).Err()
		}
		return ErrSMSCodeInvalid
	}
	_ = s.rdb.Del(ctx, codeKey, attemptsKey).Err()
	return nil
}

func (s *SMSService) codeDigest(phone, purpose, code string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	_, _ = mac.Write([]byte(phone + "\n" + purpose + "\n" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *SMSService) send(ctx context.Context, phone, purpose, code string) error {
	templateID := s.cfg.SMS.RegisterTemplateID
	if purpose == SMSPurposeResetPassword {
		templateID = s.cfg.SMS.ResetTemplateID
	}
	payload := struct {
		PhoneNumbers  []string `json:"phone_number_set"`
		TemplateID    string   `json:"template_id"`
		TemplateParams []string `json:"template_params"`
	}{
		PhoneNumbers: []string{phone},
		TemplateID: templateID,
		TemplateParams: []string{code, strconv.Itoa(s.cfg.SMS.CodeTTLMinutes)},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(s.cfg.SMS.BaseURL, "/") + "/sms/send"
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("短信接口地址无效: %w", err)
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := randomHex(16)
	if err != nil {
		return err
	}
	sign := starLoftSign("POST", u.EscapedPath(), u.RawQuery, timestamp, nonce, body, s.cfg.SMS.APISecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", s.cfg.SMS.APIKey)
	req.Header.Set("X-Sign", sign)
	req.Header.Set("X-Sign-Version", "hmac_sha256_v2")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("短信发送失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("读取短信响应失败: %w", err)
	}
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("短信服务响应异常: HTTP %d", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || result.Code != 0 {
		return fmt.Errorf("短信服务拒绝请求: code=%d message=%s", result.Code, result.Message)
	}
	return nil
}

func starLoftSign(method, path, rawQuery, timestamp, nonce string, body []byte, secret string) string {
	bodyHash := sha256.Sum256(body)
	canonical := strings.Join([]string{method, path, rawQuery, timestamp, nonce, hex.EncodeToString(bodyHash[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

func validSMSPurpose(purpose string) bool {
	return purpose == SMSPurposeRegister || purpose == SMSPurposeResetPassword
}

func randomDigits(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = '0' + b[i]%10
	}
	return string(b), nil
}
