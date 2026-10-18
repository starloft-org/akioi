package database

import (
	"fmt"
	"time"

	"akioi/internal/config"
	"akioi/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DSN 构造评测节点本地数据库的连接串。
func DSN(cfg *config.Config) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
}

// Connect 建立连接、配置连接池，并建好本地任务表。
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
	if err := db.AutoMigrate(&model.JudgeJob{}); err != nil {
		return nil, fmt.Errorf("初始化评测任务表失败: %w", err)
	}
	return db, nil
}
