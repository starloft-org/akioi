package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// DBName 为固定库名，与主站 docker-compose 的 MYSQL_DATABASE 保持一致。
const DBName = "akioi"

// JudgeConfig 为评测沙箱、队列与资源限制相关配置。
type JudgeConfig struct {
	Concurrency       int
	QueueKey          string
	TaskTimeoutSec    int
	SandboxImage      string
	DockerBin         string
	TestdataDir       string
	HostTestdataDir   string
	WorkDir           string
	HostWorkDir       string
	CompileTimeoutSec int
	CompileMemoryMB   int
	OutputLimitMB     int
	TmpfsMB           int
	CppCompiler       string
	CppStd            string
	CppFlags          string
	CCompiler         string
	CStd              string
	CFlags            string
}

// Config 为评测节点配置，仅含连接主站与执行评测所需的项。
type Config struct {
	DBHost        string
	DBPort        int
	DBUser        string
	DBPassword    string
	DBMaxOpenConn int
	DBMaxIdleConn int

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	Judge JudgeConfig

	LogDir   string
	LogMaxMB int
}

// Load 读取环境变量构建配置；已注入的变量优先于 .env 文件。
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DBHost:        getEnv("DB_HOST", "127.0.0.1"),
		DBPort:        getEnvInt("DB_PORT", 3306),
		DBUser:        getEnv("DB_USER", "root"),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBMaxOpenConn: getEnvInt("DB_MAX_OPEN_CONNS", 20),
		DBMaxIdleConn: getEnvInt("DB_MAX_IDLE_CONNS", 5),

		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		Judge: JudgeConfig{
			Concurrency:       getEnvInt("JUDGE_CONCURRENCY", 2),
			QueueKey:          getEnv("JUDGE_QUEUE_KEY", "akioi:judge:queue"),
			TaskTimeoutSec:    getEnvInt("JUDGE_TASK_TIMEOUT_SEC", 300),
			SandboxImage:      getEnv("SANDBOX_IMAGE", "akioi-sandbox:latest"),
			DockerBin:         getEnv("DOCKER_BIN", "docker"),
			TestdataDir:       getEnv("TESTDATA_DIR", "/app/testdata"),
			HostTestdataDir:   getEnv("HOST_TESTDATA_DIR", ""),
			WorkDir:           getEnv("WORK_DIR", "/app/work"),
			HostWorkDir:       getEnv("HOST_WORK_DIR", ""),
			CompileTimeoutSec: getEnvInt("COMPILE_TIMEOUT_SEC", 20),
			CompileMemoryMB:   getEnvInt("COMPILE_MEMORY_MB", 1024),
			OutputLimitMB:     getEnvInt("OUTPUT_LIMIT_MB", 64),
			TmpfsMB:           getEnvInt("TMPFS_MB", 64),
			CppCompiler:       getEnv("CPP_COMPILER", "g++"),
			CppStd:            getEnv("CPP_STD", "c++14"),
			CppFlags:          getEnv("CPP_FLAGS", "-O2"),
			CCompiler:         getEnv("C_COMPILER", "gcc"),
			CStd:              getEnv("C_STD", "c11"),
			CFlags:            getEnv("C_FLAGS", "-O2"),
		},

		LogDir:   getEnv("LOG_DIR", "/app/logs"),
		LogMaxMB: getEnvInt("LOG_MAX_MB", 100),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD 未配置")
	}
	if c.RedisAddr == "" {
		return fmt.Errorf("REDIS_ADDR 未配置")
	}
	if c.Judge.Concurrency < 1 {
		c.Judge.Concurrency = 1
	}
	return nil
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
