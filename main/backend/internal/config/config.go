package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// DBName 为固定库名，与 docker-compose 的 MYSQL_DATABASE 保持一致。
const DBName = "akioi"

type WeChatConfig struct {
	// 公众号网页授权（snsapi_userinfo）：PC 端展示自绘二维码，手机扫码后在微信内完成授权
	AppID     string
	AppSecret string
}

// SMSConfig 仅保存 StarLoft 的密钥；可热更新的业务参数保存在数据库中。
type SMSConfig struct {
	APIKey    string
	APISecret string
}

// SMTPConfig 是邮件发送的连接配置，凭据保存在 .env，不进数据库。
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// CaptchaConfig 是行为验证码的 API 凭据；留空时回退到短信密钥（同一平台账号）。
type CaptchaConfig struct {
	APIKey    string
	APISecret string
}

// JudgeConfig 为主站侧评测相关配置：投递队列、测试数据存储与评测节点同步。
type JudgeConfig struct {
	QueueKey    string
	TestdataDir string
	SyncSecret  string
}

type Config struct {
	Env        string
	ServerAddr string

	// TrustedProxies 只信任这些代理传来的转发头；直连请求忽略 X-Forwarded-For，
	// 避免客户端伪造来源 IP 绕过按 IP 的限流与访问日志。
	TrustedProxies []string

	DBHost        string
	DBPort        int
	DBUser        string
	DBPassword    string
	DBMaxOpenConn int
	DBMaxIdleConn int

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	JWTSecret string

	WeChat  WeChatConfig
	SMS     SMSConfig
	SMTP    SMTPConfig
	Captcha CaptchaConfig

	Judge JudgeConfig

	LogDir   string
	LogMaxMB int

	UploadDir string

	// BackupDir 是数据库逻辑备份的存放目录，后台「数据备份」页面在这里读写 .sql.gz 文件。
	BackupDir string
}

// Load 读取环境变量构建配置；已注入的变量优先于 .env 文件。
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Env:        getEnv("APP_ENV", "production"),
		ServerAddr: getEnv("SERVER_ADDR", ":8080"),
		// 默认只信任本机与容器/内网地址：反向代理在同网段时才能透传真实 IP。
		TrustedProxies: splitList(getEnv("TRUSTED_PROXIES", "127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")),

		DBHost:        getEnv("DB_HOST", "mysql"),
		DBPort:        getEnvInt("DB_PORT", 3306),
		DBUser:        getEnv("DB_USER", "root"),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBMaxOpenConn: getEnvInt("DB_MAX_OPEN_CONNS", 50),
		DBMaxIdleConn: getEnvInt("DB_MAX_IDLE_CONNS", 10),

		RedisAddr:     getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		JWTSecret: getEnv("JWT_SECRET", ""),

		WeChat: WeChatConfig{
			AppID:     getEnv("WECHAT_APPID", ""),
			AppSecret: getEnv("WECHAT_APPSECRET", ""),
		},
		SMS: SMSConfig{
			APIKey:    getEnv("SMS_API_KEY", ""),
			APISecret: getEnv("SMS_API_SECRET", ""),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", ""),
			Port:     getEnvInt("SMTP_PORT", 465),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", ""),
		},
		Captcha: CaptchaConfig{
			APIKey:    getEnv("CAPTCHA_API_KEY", ""),
			APISecret: getEnv("CAPTCHA_API_SECRET", ""),
		},

		Judge: JudgeConfig{
			QueueKey:    getEnv("JUDGE_QUEUE_KEY", "akioi:judge:queue"),
			TestdataDir: getEnv("TESTDATA_DIR", "/app/testdata"),
			SyncSecret:  getEnv("JUDGE_SYNC_SECRET", ""),
		},

		LogDir:   getEnv("LOG_DIR", "/app/logs"),
		LogMaxMB: getEnvInt("LOG_MAX_MB", 100),

		UploadDir: getEnv("UPLOAD_DIR", "/app/uploads"),

		BackupDir: getEnv("BACKUP_DIR", "/app/backups"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	// 登录密钥等第三方凭据保存在数据库中，首次启动时自动生成或从环境变量导入，因此这里不再要求 JWT_SECRET。
	if c.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD 未配置")
	}
	if c.RedisAddr == "" {
		return fmt.Errorf("REDIS_ADDR 未配置")
	}
	return nil
}

// EnvOr 返回环境变量值，未配置或为空时返回 def。
func EnvOr(key, def string) string {
	return getEnv(key, def)
}

func getEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// splitList 把逗号分隔的配置值拆成去空的列表。
func splitList(raw string) []string {
	out := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}
