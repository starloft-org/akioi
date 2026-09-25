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
	Enabled     bool
	AppID       string
	AppSecret   string
	RedirectURI string
}

// JudgeConfig 为主站侧评测相关配置：投递队列、测试数据存储与评测节点同步。
type JudgeConfig struct {
	QueueKey     string
	TestdataDir  string
	MaxTestcases int

	// 测试数据变动后依次推送到下列评测节点
	NodeEndpoints  []string
	SyncSecret     string
	SyncTimeoutSec int
}

type Config struct {
	AppName    string
	Env        string
	ServerAddr string

	DBHost        string
	DBPort        int
	DBUser        string
	DBPassword    string
	DBMaxOpenConn int
	DBMaxIdleConn int

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	JWTSecret   string
	JWTTTLHours int

	RegisterEnabled    bool
	RegisterInviteCode string

	WeChat WeChatConfig

	RateLimitPerMinute int
	SubmitIntervalSec  int

	Judge JudgeConfig

	LogDir   string
	LogMaxMB int

	UploadDir     string
	MaxAvatarMB   int
	MaxTestdataMB int

	CORSAllowOrigins []string
}

// Load 读取环境变量构建配置；已注入的变量优先于 .env 文件。
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		AppName:    getEnv("APP_NAME", "AKIOI"),
		Env:        getEnv("APP_ENV", "production"),
		ServerAddr: getEnv("SERVER_ADDR", ":8080"),

		DBHost:        getEnv("DB_HOST", "mysql"),
		DBPort:        getEnvInt("DB_PORT", 3306),
		DBUser:        getEnv("DB_USER", "root"),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBMaxOpenConn: getEnvInt("DB_MAX_OPEN_CONNS", 50),
		DBMaxIdleConn: getEnvInt("DB_MAX_IDLE_CONNS", 10),

		RedisAddr:     getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		JWTSecret:   getEnv("JWT_SECRET", ""),
		JWTTTLHours: getEnvInt("JWT_TTL_HOURS", 168),

		RegisterEnabled:    getEnvBool("REGISTER_ENABLED", true),
		RegisterInviteCode: getEnv("REGISTER_INVITE_CODE", ""),

		WeChat: WeChatConfig{
			Enabled:     getEnvBool("WECHAT_ENABLED", false),
			AppID:       getEnv("WECHAT_APPID", ""),
			AppSecret:   getEnv("WECHAT_APPSECRET", ""),
			RedirectURI: getEnv("WECHAT_REDIRECT_URI", ""),
		},

		RateLimitPerMinute: getEnvInt("RATE_LIMIT_PER_MINUTE", 300),
		SubmitIntervalSec:  getEnvInt("SUBMIT_INTERVAL_SEC", 10),

		Judge: JudgeConfig{
			QueueKey:     getEnv("JUDGE_QUEUE_KEY", "akioi:judge:queue"),
			TestdataDir:  getEnv("TESTDATA_DIR", "/app/testdata"),
			MaxTestcases: getEnvInt("MAX_TESTCASES", 100),

			NodeEndpoints:  splitAndTrim(getEnv("JUDGE_NODE_ENDPOINTS", "")),
			SyncSecret:     getEnv("JUDGE_SYNC_SECRET", ""),
			SyncTimeoutSec: getEnvInt("JUDGE_SYNC_TIMEOUT_SEC", 120),
		},

		LogDir:   getEnv("LOG_DIR", "/app/logs"),
		LogMaxMB: getEnvInt("LOG_MAX_MB", 100),

		UploadDir:     getEnv("UPLOAD_DIR", "/app/uploads"),
		MaxAvatarMB:   getEnvInt("MAX_AVATAR_MB", 2),
		MaxTestdataMB: getEnvInt("MAX_TESTDATA_MB", 512),

		CORSAllowOrigins: splitAndTrim(getEnv("CORS_ALLOW_ORIGINS", "")),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET 未配置")
	}
	if c.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD 未配置")
	}
	if c.RedisAddr == "" {
		return fmt.Errorf("REDIS_ADDR 未配置")
	}
	if len(c.Judge.NodeEndpoints) > 0 && c.Judge.SyncSecret == "" {
		return fmt.Errorf("配置了 JUDGE_NODE_ENDPOINTS 时必须同时配置 JUDGE_SYNC_SECRET")
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

func getEnvFloat(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

// getEnvBool 兼容 1/0 与 true/false 两种写法。
func getEnvBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "":
		return def
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

func splitAndTrim(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
