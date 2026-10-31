package database

import (
	"fmt"
	"time"

	"akioi/internal/config"
	"akioi/internal/logger"
	"akioi/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DSN 构造业务连接串。
func DSN(cfg *config.Config) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, config.DBName)
}

// MigrateDSN 构造迁移连接串，需 multiStatements 支持单连接批量执行。
func MigrateDSN(cfg *config.Config) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, config.DBName)
}

// Connect 建立连接并配置连接池。
func Connect(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(DSN(cfg)), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpenConn)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConn)
	sqlDB.SetConnMaxLifetime(time.Hour)
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// AutoMigrate 补齐表结构。
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.User{},
		&model.OAuthBinding{},
		&model.Problem{},
		&model.ProblemBank{},
		&model.Testcase{},
		&model.Submission{},
		&model.Contest{},
		&model.ContestProblem{},
		&model.ContestParticipant{},
		&model.Announcement{},
		&model.Solution{},
		&model.Discussion{},
		&model.DiscussionComment{},
		&model.SystemSetting{},
		&model.Team{},
		&model.TeamMember{},
		&model.TeamApplication{},
		&model.TeamProblemSet{},
		&model.UserProblemSet{},
	); err != nil {
		return err
	}

	// 历史版本中的出题人角色已取消，降级为普通用户以避免意外提权。
	return db.Model(&model.User{}).Where("role = ?", "setter").Update("role", model.RoleUser).Error
}

// Seed 幂等写入初始数据：仅在没有任何管理员时创建默认管理员。
func Seed(db *gorm.DB, cfg *config.Config) error {
	var count int64
	if err := db.Model(&model.User{}).Where("role = ?", model.RoleAdmin).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	username := getEnvDefault("ADMIN_INIT_USERNAME", "admin")
	password := config.EnvOr("ADMIN_INIT_PASSWORD", "")
	if len(password) < 8 || len(password) > 64 {
		return fmt.Errorf("首次启动时必须在 .env 配置 8 到 64 位的 ADMIN_INIT_PASSWORD")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin := model.User{
		Username:     username,
		PasswordHash: string(hash),
		Nickname:     "管理员",
		Role:         model.RoleAdmin,
		Status:       model.UserStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		return err
	}
	logger.Businessf("已创建默认管理员 username=%s，请登录后立即修改密码", username)
	return nil
}

func getEnvDefault(key, def string) string {
	v := config.EnvOr(key, "")
	if v == "" {
		return def
	}
	return v
}
