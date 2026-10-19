package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"akioi/internal/model"

	"gorm.io/gorm"
)

// themeColorPattern 校验六位十六进制颜色值（#RRGGBB）。
var themeColorPattern = regexp.MustCompile(`^#[0-9A-F]{6}$`)

const runtimeSettingsKey = "runtime"

// RuntimeSettings 是可在后台修改并立即生效的非敏感业务设置。
type RuntimeSettings struct {
	AppName              string   `json:"app_name"`
	ThemeColor           string   `json:"theme_color"`
	JWTTTLHours          int      `json:"jwt_ttl_hours"`
	RegisterEnabled      bool     `json:"register_enabled"`
	WeChatEnabled        bool     `json:"wechat_enabled"`
	WeChatRedirectURI    string   `json:"wechat_redirect_uri"`
	SMSEnabled           bool     `json:"sms_enabled"`
	EmailEnabled         bool     `json:"email_enabled"`
	SMSAPIBaseURL        string   `json:"sms_api_base_url"`
	SMSRegisterTemplate  string   `json:"sms_register_template_id"`
	SMSResetTemplate     string   `json:"sms_reset_template_id"`
	SMSCodeTTLMinutes    int      `json:"sms_code_ttl_minutes"`
	SMSSendIntervalSec   int      `json:"sms_send_interval_sec"`
	SMSRequestTimeoutSec int      `json:"sms_request_timeout_sec"`
	RateLimitPerMinute   int      `json:"rate_limit_per_minute"`
	SubmitIntervalSec    int      `json:"submit_interval_sec"`
	MaxTestcases         int      `json:"max_testcases"`
	JudgeNodeEndpoints   []string `json:"judge_node_endpoints"`
	JudgeSyncTimeoutSec  int      `json:"judge_sync_timeout_sec"`
	MaxAvatarMB          int      `json:"max_avatar_mb"`
	MaxTestdataMB        int      `json:"max_testdata_mb"`
	CORSAllowOrigins     []string `json:"cors_allow_origins"`
}

func DefaultRuntimeSettings() RuntimeSettings {
	return RuntimeSettings{
		AppName:              "AKIOI",
		ThemeColor:           "#1CD5C7",
		JWTTTLHours:           168,
		RegisterEnabled:      true,
		WeChatEnabled:        false,
		SMSEnabled:           false,
		EmailEnabled:         false,
		SMSAPIBaseURL:        "https://api.starloft.cn/v2",
		SMSCodeTTLMinutes:    5,
		SMSSendIntervalSec:   60,
		SMSRequestTimeoutSec: 10,
		RateLimitPerMinute:   300,
		SubmitIntervalSec:    10,
		MaxTestcases:         100,
		JudgeSyncTimeoutSec:  120,
		MaxAvatarMB:          2,
		MaxTestdataMB:        512,
		JudgeNodeEndpoints:   []string{},
		CORSAllowOrigins:     []string{},
	}
}

type SettingService struct {
	db      *gorm.DB
	current atomic.Value
}

// NewSettingService 从数据库加载设置；首次启动会写入默认值。
func NewSettingService(db *gorm.DB) (*SettingService, error) {
	s := &SettingService{db: db}
	settings := DefaultRuntimeSettings()
	var row model.SystemSetting
	needsSave := false
	err := db.Where("setting_key = ?", runtimeSettingsKey).First(&row).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// 升级旧部署时仅执行一次：把旧 .env 中的业务项写入数据库，避免升级后设置丢失。
		settings = legacyEnvironmentSettings(settings)
		needsSave = true
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal([]byte(row.Value), &settings); err != nil {
			return nil, fmt.Errorf("解析数据库运行时设置失败: %w", err)
		}
	}
	settings, err = NormalizeRuntimeSettings(settings)
	if err != nil {
		return nil, fmt.Errorf("数据库运行时设置无效: %w", err)
	}
	if needsSave {
		if err := s.save(settings); err != nil {
			return nil, err
		}
	}
	s.current.Store(settings)
	return s, nil
}

// NewStaticSettingService 供测试或离线装配使用，不读写数据库。
func NewStaticSettingService(settings RuntimeSettings) *SettingService {
	normalized, err := NormalizeRuntimeSettings(settings)
	if err != nil {
		panic(err)
	}
	s := &SettingService{}
	s.current.Store(normalized)
	return s
}

// Current 返回当前不可变快照的副本。
func (s *SettingService) Current() RuntimeSettings {
	settings := s.current.Load().(RuntimeSettings)
	settings.JudgeNodeEndpoints = append([]string{}, settings.JudgeNodeEndpoints...)
	settings.CORSAllowOrigins = append([]string{}, settings.CORSAllowOrigins...)
	return settings
}

// Update 先持久化再切换内存快照，成功后新请求立即读取到新值。
func (s *SettingService) Update(settings RuntimeSettings) (RuntimeSettings, error) {
	normalized, err := NormalizeRuntimeSettings(settings)
	if err != nil {
		return RuntimeSettings{}, err
	}
	if s.db == nil {
		return RuntimeSettings{}, fmt.Errorf("设置存储不可用")
	}
	if err := s.save(normalized); err != nil {
		return RuntimeSettings{}, err
	}
	s.current.Store(normalized)
	return s.Current(), nil
}

func (s *SettingService) save(settings RuntimeSettings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	row := model.SystemSetting{Name: runtimeSettingsKey, Value: string(raw)}
	return s.db.Save(&row).Error
}

func NormalizeRuntimeSettings(s RuntimeSettings) (RuntimeSettings, error) {
	s.AppName = strings.TrimSpace(s.AppName)
	if s.AppName == "" || len([]rune(s.AppName)) > 64 {
		return s, fmt.Errorf("站点名称长度必须为 1 到 64 个字符")
	}
	s.ThemeColor = strings.ToUpper(strings.TrimSpace(s.ThemeColor))
	if s.ThemeColor == "" {
		s.ThemeColor = DefaultRuntimeSettings().ThemeColor
	}
	if !themeColorPattern.MatchString(s.ThemeColor) {
		return s, fmt.Errorf("主题色格式不正确，应形如 #1CD5C7")
	}
	if s.JWTTTLHours < 1 || s.JWTTTLHours > 8760 {
		return s, fmt.Errorf("登录有效期必须为 1 到 8760 小时")
	}
	if s.RateLimitPerMinute < 0 || s.RateLimitPerMinute > 100000 {
		return s, fmt.Errorf("每分钟请求限制必须为 0 到 100000")
	}
	if s.SubmitIntervalSec < 0 || s.SubmitIntervalSec > 3600 {
		return s, fmt.Errorf("提交间隔必须为 0 到 3600 秒")
	}
	if s.MaxTestcases < 1 || s.MaxTestcases > 10000 {
		return s, fmt.Errorf("单题测试点上限必须为 1 到 10000")
	}
	if s.JudgeSyncTimeoutSec < 1 || s.JudgeSyncTimeoutSec > 3600 {
		return s, fmt.Errorf("评测节点同步超时必须为 1 到 3600 秒")
	}
	if s.MaxAvatarMB < 1 || s.MaxAvatarMB > 100 {
		return s, fmt.Errorf("头像上限必须为 1 到 100 MB")
	}
	if s.MaxTestdataMB < 1 || s.MaxTestdataMB > 10240 {
		return s, fmt.Errorf("测试数据上限必须为 1 到 10240 MB")
	}
	if s.SMSCodeTTLMinutes < 1 || s.SMSCodeTTLMinutes > 30 {
		return s, fmt.Errorf("短信验证码有效期必须为 1 到 30 分钟")
	}
	if s.SMSSendIntervalSec < 30 || s.SMSSendIntervalSec > 3600 {
		return s, fmt.Errorf("短信发送间隔必须为 30 到 3600 秒")
	}
	if s.SMSRequestTimeoutSec < 1 || s.SMSRequestTimeoutSec > 120 {
		return s, fmt.Errorf("短信请求超时必须为 1 到 120 秒")
	}

	s.SMSAPIBaseURL = strings.TrimRight(strings.TrimSpace(s.SMSAPIBaseURL), "/")
	if err := validateHTTPURL(s.SMSAPIBaseURL, false, "短信接口地址"); err != nil {
		return s, err
	}
	s.SMSRegisterTemplate = strings.TrimSpace(s.SMSRegisterTemplate)
	s.SMSResetTemplate = strings.TrimSpace(s.SMSResetTemplate)
	if len(s.SMSRegisterTemplate) > 128 || len(s.SMSResetTemplate) > 128 {
		return s, fmt.Errorf("短信模板 ID 不能超过 128 个字符")
	}
	if s.SMSEnabled && (s.SMSRegisterTemplate == "" || s.SMSResetTemplate == "") {
		return s, fmt.Errorf("启用短信服务前必须填写注册和密码重置模板 ID")
	}
	s.WeChatRedirectURI = strings.TrimSpace(s.WeChatRedirectURI)
	if err := validateHTTPURL(s.WeChatRedirectURI, true, "微信回调地址"); err != nil {
		return s, err
	}
	if s.WeChatEnabled && s.WeChatRedirectURI == "" {
		return s, fmt.Errorf("启用微信登录前必须填写微信回调地址")
	}

	var err error
	s.JudgeNodeEndpoints, err = normalizeURLs(s.JudgeNodeEndpoints, false, "评测节点地址")
	if err != nil {
		return s, err
	}
	s.CORSAllowOrigins, err = normalizeOrigins(s.CORSAllowOrigins)
	if err != nil {
		return s, err
	}
	return s, nil
}

func validateHTTPURL(raw string, allowEmpty bool, label string) error {
	if raw == "" && allowEmpty {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s必须是完整的 http 或 https 地址", label)
	}
	return nil
}

func normalizeURLs(values []string, allowStar bool, label string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value == "" || seen[value] {
			continue
		}
		if allowStar && value == "*" {
			out = append(out, value)
			seen[value] = true
			continue
		}
		if err := validateHTTPURL(value, false, label); err != nil {
			return nil, err
		}
		out = append(out, value)
		seen[value] = true
	}
	return out, nil
}

func normalizeOrigins(values []string) ([]string, error) {
	return normalizeURLs(values, true, "跨域来源")
}

func legacyEnvironmentSettings(s RuntimeSettings) RuntimeSettings {
	legacyString("APP_NAME", &s.AppName)
	legacyInt("JWT_TTL_HOURS", &s.JWTTTLHours)
	legacyBool("REGISTER_ENABLED", &s.RegisterEnabled)
	legacyBool("WECHAT_ENABLED", &s.WeChatEnabled)
	legacyString("WECHAT_REDIRECT_URI", &s.WeChatRedirectURI)
	legacyString("SMS_API_BASE_URL", &s.SMSAPIBaseURL)
	legacyString("SMS_REGISTER_TEMPLATE_ID", &s.SMSRegisterTemplate)
	legacyString("SMS_RESET_TEMPLATE_ID", &s.SMSResetTemplate)
	legacyInt("SMS_CODE_TTL_MINUTES", &s.SMSCodeTTLMinutes)
	legacyInt("SMS_SEND_INTERVAL_SEC", &s.SMSSendIntervalSec)
	legacyInt("SMS_REQUEST_TIMEOUT_SEC", &s.SMSRequestTimeoutSec)
	legacyInt("RATE_LIMIT_PER_MINUTE", &s.RateLimitPerMinute)
	legacyInt("SUBMIT_INTERVAL_SEC", &s.SubmitIntervalSec)
	legacyInt("MAX_TESTCASES", &s.MaxTestcases)
	legacyInt("JUDGE_SYNC_TIMEOUT_SEC", &s.JudgeSyncTimeoutSec)
	legacyInt("MAX_AVATAR_MB", &s.MaxAvatarMB)
	legacyInt("MAX_TESTDATA_MB", &s.MaxTestdataMB)
	s.JudgeNodeEndpoints = legacyList("JUDGE_NODE_ENDPOINTS", s.JudgeNodeEndpoints)
	s.CORSAllowOrigins = legacyList("CORS_ALLOW_ORIGINS", s.CORSAllowOrigins)

	// 旧版本没有独立短信开关，凭据与两个模板均存在时视为原本已启用。
	if strings.TrimSpace(os.Getenv("SMS_API_KEY")) != "" && strings.TrimSpace(os.Getenv("SMS_API_SECRET")) != "" &&
		s.SMSRegisterTemplate != "" && s.SMSResetTemplate != "" {
		s.SMSEnabled = true
	}
	return s
}

func legacyString(key string, target *string) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		*target = value
	}
}

func legacyInt(key string, target *int) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			*target = parsed
		}
	}
}

func legacyBool(key string, target *bool) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		*target = true
	case "0", "false", "no", "off", "2":
		*target = false
	}
}

func legacyList(key string, current []string) []string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return current
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
