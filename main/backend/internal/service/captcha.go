package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"akioi/internal/config"
)

const (
	// CaptchaClientTypeWeb 表示 Web 端验证码应用。
	CaptchaClientTypeWeb = "web"
	// CaptchaSceneAccount 用于登录、注册等账号操作。
	CaptchaSceneAccount = "account"
	// CaptchaSceneSMS 用于发送短信/邮件验证码。
	CaptchaSceneSMS = "sms"
	// CaptchaVerifyTypeSlide 为平台默认的滑动拼图验证方式。
	CaptchaVerifyTypeSlide = "slide"

	captchaAPIBase = "https://api.starloft.cn/v2/captcha"
	// captchaJSFallback 是平台未下发 JS 地址时的兜底，文档要求动态引入、不可本地缓存。
	captchaJSFallback = "https://service.starloft.cn/service/captcha/js"
	captchaConfigTTL  = 10 * time.Minute
)

var (
	// ErrCaptchaInvalid 表示票据校验未通过（过期、重复使用、答案不匹配等）。
	ErrCaptchaInvalid = errors.New("人机验证未通过，请重新完成验证")
	// ErrCaptchaMissing 表示请求未携带人机验证结果，多为前端未加载验证码组件。
	ErrCaptchaMissing = errors.New("请先完成人机验证；若页面未出现验证框，请刷新页面后重试")
	// ErrCaptchaUnavailable 表示验证码平台不可用或额度不足。
	ErrCaptchaUnavailable = errors.New("人机验证服务暂不可用，请稍后再试")
	// ErrCaptchaNotEnabled 表示后端未开启人机验证。
	ErrCaptchaNotEnabled = errors.New("人机验证未开启")
)

// CaptchaConfig 是前端渲染验证码所需的公开配置，不含任何密钥。
type CaptchaConfig struct {
	Enabled bool   `json:"enabled"`
	AppID   string `json:"captcha_app_id"`
	JSURL   string `json:"web_js_url"`
	Scene   string `json:"scene"`
}

type captchaCachedConfig struct {
	value     CaptchaConfig
	expiresAt time.Time
}

// CaptchaService 对接 StarLoft 行为验证码：前端取 ticket/randstr，服务端校验票据。
type CaptchaService struct {
	cfg      *config.Config
	settings *SettingService
	client   *http.Client

	mu     sync.Mutex
	cached map[string]captchaCachedConfig
}

func NewCaptcha(cfg *config.Config, settings *SettingService) *CaptchaService {
	return &CaptchaService{
		cfg:      cfg,
		settings: settings,
		client:   &http.Client{Timeout: 8 * time.Second},
		cached:   make(map[string]captchaCachedConfig),
	}
}

// apiKey/apiSecret 优先使用验证码专用密钥，未配置时回退到短信密钥（同一平台账号）。
func (s *CaptchaService) apiKey() string {
	secrets := s.settings.Secrets()
	if secrets.CaptchaAPIKey != "" {
		return secrets.CaptchaAPIKey
	}
	return secrets.SMSAPIKey
}

func (s *CaptchaService) apiSecret() string {
	secrets := s.settings.Secrets()
	if secrets.CaptchaAPISecret != "" {
		return secrets.CaptchaAPISecret
	}
	return secrets.SMSAPISecret
}

// Enabled 需要后台开关与 API 密钥同时就绪。
func (s *CaptchaService) Enabled() bool {
	return s.settings.Current().CaptchaEnabled && s.apiKey() != "" && s.apiSecret() != ""
}

// Config 返回前端验证码配置（平台免费接口），按场景缓存 10 分钟。
func (s *CaptchaService) Config(ctx context.Context, scene string) (CaptchaConfig, error) {
	if !s.Enabled() {
		return CaptchaConfig{}, ErrCaptchaNotEnabled
	}
	if scene == "" {
		scene = CaptchaSceneAccount
	}

	s.mu.Lock()
	if item, ok := s.cached[scene]; ok && time.Now().Before(item.expiresAt) {
		s.mu.Unlock()
		return item.value, nil
	}
	s.mu.Unlock()

	query := url.Values{}
	query.Set("client_type", CaptchaClientTypeWeb)
	query.Set("scene", scene)
	query.Set("verify_type", CaptchaVerifyTypeSlide)
	rawQuery := query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, captchaAPIBase+"/config?"+rawQuery, nil)
	if err != nil {
		return CaptchaConfig{}, err
	}
	if err := s.sign(req, http.MethodGet, "/v2/captcha/config", rawQuery, nil); err != nil {
		return CaptchaConfig{}, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return CaptchaConfig{}, fmt.Errorf("%w: %v", ErrCaptchaUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			CaptchaAppID string `json:"captcha_app_id"`
			WebJSURL     string `json:"web_js_url"`
			Enabled      bool   `json:"enabled"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return CaptchaConfig{}, ErrCaptchaUnavailable
	}
	if resp.StatusCode != http.StatusOK || out.Code != 0 || out.Data.CaptchaAppID == "" {
		return CaptchaConfig{}, fmt.Errorf("%w: %s", ErrCaptchaUnavailable, strings.TrimSpace(out.Message))
	}

	cfg := CaptchaConfig{
		Enabled: out.Data.Enabled,
		AppID:   out.Data.CaptchaAppID,
		JSURL:   out.Data.WebJSURL,
		Scene:   scene,
	}
	if cfg.JSURL == "" {
		cfg.JSURL = captchaJSFallback
	}

	s.mu.Lock()
	s.cached[scene] = captchaCachedConfig{value: cfg, expiresAt: time.Now().Add(captchaConfigTTL)}
	s.mu.Unlock()
	return cfg, nil
}

// Verify 校验前端取得的票据；票据 5 分钟有效且只能校验一次。
func (s *CaptchaService) Verify(ctx context.Context, scene, ticket, randstr, ip string) error {
	if !s.Enabled() {
		return nil
	}
	ticket = strings.TrimSpace(ticket)
	randstr = strings.TrimSpace(randstr)
	if ticket == "" || randstr == "" {
		return ErrCaptchaMissing
	}
	if scene == "" {
		scene = CaptchaSceneAccount
	}

	payload := struct {
		ClientType string `json:"client_type"`
		Scene      string `json:"scene"`
		VerifyType string `json:"verify_type"`
		Ticket     string `json:"ticket"`
		RandStr    string `json:"randstr"`
		IP         string `json:"ip,omitempty"`
	}{
		ClientType: CaptchaClientTypeWeb,
		Scene:      scene,
		VerifyType: CaptchaVerifyTypeSlide,
		Ticket:     ticket,
		RandStr:    randstr,
		IP:         strings.TrimSpace(ip),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, captchaAPIBase+"/verify", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := s.sign(req, http.MethodPost, "/v2/captcha/verify", "", body); err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCaptchaUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Verified    bool   `json:"verified"`
			CaptchaCode string `json:"captcha_code"`
			CaptchaMsg  string `json:"captcha_msg"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return ErrCaptchaUnavailable
	}
	if resp.StatusCode == http.StatusPaymentRequired {
		return fmt.Errorf("%w: 账户额度不足", ErrCaptchaUnavailable)
	}
	if resp.StatusCode != http.StatusOK || out.Code != 0 {
		return fmt.Errorf("%w: %s", ErrCaptchaUnavailable, strings.TrimSpace(out.Message))
	}
	if !out.Data.Verified {
		return ErrCaptchaInvalid
	}
	return nil
}

// sign 按平台规则写入鉴权头：API Key + HMAC-SHA256 签名。
// 必须与短信服务一致携带 X-Sign-Version / X-Timestamp / X-Nonce，
// 否则平台会以「unsupported sign version」直接拒绝。
func (s *CaptchaService) sign(req *http.Request, method, path, rawQuery string, body []byte) error {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := randomHex(16)
	if err != nil {
		return err
	}
	sign := starLoftSign(method, path, rawQuery, timestamp, nonce, body, s.apiSecret())
	req.Header.Set("X-Api-Key", s.apiKey())
	req.Header.Set("X-Sign", sign)
	req.Header.Set("X-Sign-Version", "hmac_sha256_v2")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	return nil
}
