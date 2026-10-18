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
	AppID     string
	AppSecret string
}

// SMSConfig 仅保存 StarLoft 的密钥；可热更新的业务参数保存在数据库中。
type SMSConfig struct {
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

	WeChat WeChatConfig
	SMS    SMSConfig

	Judge JudgeConfig

	LogDir   string
	LogMaxMB int

	UploadDir string
}

// Load 读取环境变量构建配置；已注入的变量优先于 .env 文件。
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
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

		JWTSecret: getEnv("JWT_SECRET", ""),

		WeChat: WeChatConfig{
			AppID:     getEnv("WECHAT_APPID", ""),
			AppSecret: getEnv("WECHAT_APPSECRET", ""),
		},
		SMS: SMSConfig{
			APIKey:    getEnv("SMS_API_KEY", ""),
			APISecret: getEnv("SMS_API_SECRET", ""),
		},

		Judge: JudgeConfig{
			QueueKey:    getEnv("JUDGE_QUEUE_KEY", "akioi:judge:queue"),
			TestdataDir: getEnv("TESTDATA_DIR", "/app/testdata"),
			SyncSecret:  getEnv("JUDGE_SYNC_SECRET", ""),
		},

		LogDir:   getEnv("LOG_DIR", "/app/logs"),
		LogMaxMB: getEnvInt("LOG_MAX_MB", 100),

		UploadDir: getEnv("UPLOAD_DIR", "/app/uploads"),
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
