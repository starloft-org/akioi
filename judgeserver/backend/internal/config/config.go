package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// 评测节点使用独立数据库，默认库名与主站区分开。
const DefaultDBName = "akioi_judge"

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

	// 控制面：接收主站推送的测试数据与评测任务
	ListenAddr    string
	SyncSecret    string
	MaxTestdataMB int
}

// Config 为评测节点配置。数据库与 Redis 都是节点自带的实例，
// 只有回传评测结果时才访问主站。
type Config struct {
	DBHost        string
	DBPort        int
	DBName        string
	DBUser        string
	DBPassword    string
	DBMaxOpenConn int
	DBMaxIdleConn int

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// MainBaseURL 是主站地址，用于回传评测结果，同机部署填宿主机内网 IP，跨机填主站地址
	MainBaseURL string

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
		DBName:        getEnv("DB_NAME", DefaultDBName),
		DBUser:        getEnv("DB_USER", "root"),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBMaxOpenConn: getEnvInt("DB_MAX_OPEN_CONNS", 20),
		DBMaxIdleConn: getEnvInt("DB_MAX_IDLE_CONNS", 5),

		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		MainBaseURL: strings.TrimRight(getEnv("MAIN_BASE_URL", ""), "/"),

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

			ListenAddr:    getEnv("JUDGE_LISTEN_ADDR", ":9000"),
			SyncSecret:    getEnv("JUDGE_SYNC_SECRET", ""),
			MaxTestdataMB: getEnvInt("MAX_TESTDATA_MB", 512),
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
	if c.DBName == "" {
		c.DBName = DefaultDBName
	}
	if c.RedisAddr == "" {
		return fmt.Errorf("REDIS_ADDR 未配置")
	}
	if c.Judge.Concurrency < 1 {
		c.Judge.Concurrency = 1
	}
	if c.MainBaseURL == "" {
		return fmt.Errorf("MAIN_BASE_URL 未配置，评测结果无法回传主站")
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
