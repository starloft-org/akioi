package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"akioi/internal/model"

	"gorm.io/gorm"
)

// secretSettingsKey 是密钥记录在 system_settings 表中的主键，与 runtime 分开保存。
const secretSettingsKey = "secrets"

// Secrets 保存第三方集成密钥与登录密钥，只能在管理后台修改，不随运行时设置下发。
type Secrets struct {
	JWTSecret string `json:"jwt_secret"`

	WeChatAppID     string `json:"wechat_app_id"`
	WeChatAppSecret string `json:"wechat_app_secret"`

	SMSAPIKey    string `json:"sms_api_key"`
	SMSAPISecret string `json:"sms_api_secret"`

	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from"`

	CaptchaAPIKey    string `json:"captcha_api_key"`
	CaptchaAPISecret string `json:"captcha_api_secret"`

	JudgeSyncSecret string `json:"judge_sync_secret"`
}

// SecretsView 是管理后台可回填的非敏感字段；密钥本身只返回是否已配置。
type SecretsView struct {
	WeChatAppID  string `json:"wechat_app_id"`
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPFrom     string `json:"smtp_from"`
}

// SecretsPatch 是管理后台提交的增量修改：密钥字段留空表示保持不变，Clear 中的分组会被清空。
type SecretsPatch struct {
	WeChatAppID      string   `json:"wechat_app_id"`
	WeChatAppSecret  string   `json:"wechat_app_secret"`
	SMSAPIKey        string   `json:"sms_api_key"`
	SMSAPISecret     string   `json:"sms_api_secret"`
	SMTPHost         string   `json:"smtp_host"`
	SMTPPort         int      `json:"smtp_port"`
	SMTPUsername     string   `json:"smtp_username"`
	SMTPPassword     string   `json:"smtp_password"`
	SMTPFrom         string   `json:"smtp_from"`
	CaptchaAPIKey    string   `json:"captcha_api_key"`
	CaptchaAPISecret string   `json:"captcha_api_secret"`
	JudgeSyncSecret  string   `json:"judge_sync_secret"`
	Clear            []string `json:"clear"`
	RegenerateJWT    bool     `json:"regenerate_jwt"`
}

// Normalized 去掉首尾空白并补齐端口默认值。
func (s Secrets) Normalized() Secrets {
	s.JWTSecret = strings.TrimSpace(s.JWTSecret)
	s.WeChatAppID = strings.TrimSpace(s.WeChatAppID)
	s.WeChatAppSecret = strings.TrimSpace(s.WeChatAppSecret)
	s.SMSAPIKey = strings.TrimSpace(s.SMSAPIKey)
	s.SMSAPISecret = strings.TrimSpace(s.SMSAPISecret)
	s.SMTPHost = strings.TrimSpace(s.SMTPHost)
	s.SMTPUsername = strings.TrimSpace(s.SMTPUsername)
	s.SMTPPassword = strings.TrimSpace(s.SMTPPassword)
	s.SMTPFrom = strings.TrimSpace(s.SMTPFrom)
	s.CaptchaAPIKey = strings.TrimSpace(s.CaptchaAPIKey)
	s.CaptchaAPISecret = strings.TrimSpace(s.CaptchaAPISecret)
	s.JudgeSyncSecret = strings.TrimSpace(s.JudgeSyncSecret)
	if s.SMTPHost != "" && s.SMTPPort == 0 {
		s.SMTPPort = 587
	}
	return s
}

// View 返回可以在后台回填的非敏感字段。
func (s Secrets) View() SecretsView {
	return SecretsView{
		WeChatAppID:  s.WeChatAppID,
		SMTPHost:     s.SMTPHost,
		SMTPPort:     s.SMTPPort,
		SMTPUsername: s.SMTPUsername,
		SMTPFrom:     s.SMTPFrom,
	}
}

// Status 返回各集成是否已具备可用凭据。
func (s Secrets) Status() map[string]bool {
	return map[string]bool{
		"jwt_secret_configured":          s.JWTSecret != "",
		"wechat_credentials_configured":  s.WeChatAppID != "" && s.WeChatAppSecret != "",
		"sms_credentials_configured":     s.SMSAPIKey != "" && s.SMSAPISecret != "",
		"smtp_credentials_configured":    s.SMTPHost != "" && s.SMTPFrom != "" && s.SMTPPassword != "",
		"captcha_credentials_configured": s.CaptchaAPIKey != "" || s.SMSAPIKey != "",
		"judge_sync_secret_configured":   s.JudgeSyncSecret != "",
	}
}

// Merge 把后台提交的增量合并到当前密钥；Clear 中的分组会被清空，密钥字段留空表示保持不变。
func (s Secrets) Merge(patch SecretsPatch) (Secrets, error) {
	next := s.Normalized()
	// 非敏感字段允许清空，因此总是按提交值覆盖。
	next.WeChatAppID = strings.TrimSpace(patch.WeChatAppID)
	next.SMTPHost = strings.TrimSpace(patch.SMTPHost)
	next.SMTPUsername = strings.TrimSpace(patch.SMTPUsername)
	next.SMTPFrom = strings.TrimSpace(patch.SMTPFrom)
	next.SMTPPort = patch.SMTPPort

	keepOrReplace := func(current, incoming string) string {
		if incoming = strings.TrimSpace(incoming); incoming != "" {
			return incoming
		}
		return current
	}
	next.WeChatAppSecret = keepOrReplace(next.WeChatAppSecret, patch.WeChatAppSecret)
	next.SMSAPIKey = keepOrReplace(next.SMSAPIKey, patch.SMSAPIKey)
	next.SMSAPISecret = keepOrReplace(next.SMSAPISecret, patch.SMSAPISecret)
	next.SMTPPassword = keepOrReplace(next.SMTPPassword, patch.SMTPPassword)
	next.CaptchaAPIKey = keepOrReplace(next.CaptchaAPIKey, patch.CaptchaAPIKey)
	next.CaptchaAPISecret = keepOrReplace(next.CaptchaAPISecret, patch.CaptchaAPISecret)
	next.JudgeSyncSecret = keepOrReplace(next.JudgeSyncSecret, patch.JudgeSyncSecret)

	for _, group := range patch.Clear {
		switch strings.TrimSpace(group) {
		case "":
			continue
		case "wechat":
			next.WeChatAppID, next.WeChatAppSecret = "", ""
		case "sms":
			next.SMSAPIKey, next.SMSAPISecret = "", ""
		case "smtp":
			next.SMTPHost, next.SMTPPort, next.SMTPUsername, next.SMTPPassword, next.SMTPFrom = "", 0, "", "", ""
		case "captcha":
			next.CaptchaAPIKey, next.CaptchaAPISecret = "", ""
		case "judge":
			next.JudgeSyncSecret = ""
		default:
			return s, fmt.Errorf("未知的清除分组 %s", group)
		}
	}

	if patch.RegenerateJWT {
		secret, err := randomHex(32)
		if err != nil {
			return s, fmt.Errorf("生成登录密钥失败: %w", err)
		}
		next.JWTSecret = secret
	}
	if next.JWTSecret == "" {
		secret, err := randomHex(32)
		if err != nil {
			return s, fmt.Errorf("生成登录密钥失败: %w", err)
		}
		next.JWTSecret = secret
	}
	return next.Normalized(), nil
}

// Validate 校验密钥是否满足已启用的集成，settings 为合并后的运行时设置。
func (s Secrets) Validate(settings RuntimeSettings) error {
	if settings.SMSEnabled && (s.SMSAPIKey == "" || s.SMSAPISecret == "") {
		return fmt.Errorf("启用短信服务前必须先在密钥设置中填写短信 API Key 与 API Secret")
	}
	if settings.WeChatEnabled && (s.WeChatAppID == "" || s.WeChatAppSecret == "") {
		return fmt.Errorf("启用微信登录前必须先在密钥设置中填写微信 AppID 与 AppSecret")
	}
	if len(settings.JudgeNodeEndpoints) > 0 && s.JudgeSyncSecret == "" {
		return fmt.Errorf("配置评测节点前必须先在密钥设置中填写评测节点同步密钥")
	}
	return nil
}

// secretsFromEnvironment 读取旧版本 .env 中的密钥，用于升级后一次性导入数据库。
func secretsFromEnvironment() Secrets {
	value := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	port, _ := strconv.Atoi(value("SMTP_PORT"))
	return Secrets{
		JWTSecret:        value("JWT_SECRET"),
		WeChatAppID:      value("WECHAT_APPID"),
		WeChatAppSecret:  value("WECHAT_APPSECRET"),
		SMSAPIKey:        value("SMS_API_KEY"),
		SMSAPISecret:     value("SMS_API_SECRET"),
		SMTPHost:         value("SMTP_HOST"),
		SMTPPort:         port,
		SMTPUsername:     value("SMTP_USERNAME"),
		SMTPPassword:     value("SMTP_PASSWORD"),
		SMTPFrom:         value("SMTP_FROM"),
		CaptchaAPIKey:    value("CAPTCHA_API_KEY"),
		CaptchaAPISecret: value("CAPTCHA_API_SECRET"),
		JudgeSyncSecret:  value("JUDGE_SYNC_SECRET"),
	}.Normalized()
}

// loadSecrets 读取数据库中的密钥记录；记录不存在时返回 found=false。
func (s *SettingService) loadSecrets() (Secrets, bool, error) {
	var row model.SystemSetting
	err := s.db.Where("setting_key = ?", secretSettingsKey).First(&row).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return Secrets{}, false, nil
	case err != nil:
		return Secrets{}, false, err
	}
	var secrets Secrets
	if err := json.Unmarshal([]byte(row.Value), &secrets); err != nil {
		return Secrets{}, false, fmt.Errorf("解析数据库密钥设置失败: %w", err)
	}
	secrets = secrets.Normalized()
	if secrets.JWTSecret == "" {
		secret, err := randomHex(32)
		if err != nil {
			return Secrets{}, false, fmt.Errorf("生成登录密钥失败: %w", err)
		}
		secrets.JWTSecret = secret
		if err := s.saveSecrets(secrets); err != nil {
			return Secrets{}, false, err
		}
	}
	return secrets, true, nil
}

// Secrets 返回当前密钥快照的副本。
func (s *SettingService) Secrets() Secrets {
	return s.secrets.Load().(Secrets)
}

// UpdateSecrets 先持久化再切换内存快照。
func (s *SettingService) UpdateSecrets(next Secrets) (Secrets, error) {
	if s.db == nil {
		return Secrets{}, fmt.Errorf("设置存储不可用")
	}
	next = next.Normalized()
	if next.JWTSecret == "" {
		return Secrets{}, fmt.Errorf("登录密钥不能为空")
	}
	if err := s.saveSecrets(next); err != nil {
		return Secrets{}, err
	}
	s.secrets.Store(next)
	return next, nil
}

func (s *SettingService) saveSecrets(secrets Secrets) error {
	raw, err := json.Marshal(secrets)
	if err != nil {
		return err
	}
	row := model.SystemSetting{Name: secretSettingsKey, Value: string(raw)}
	return s.db.Save(&row).Error
}
