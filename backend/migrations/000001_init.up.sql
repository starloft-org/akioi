-- 000001 基线：统一数据库字符集与排序规则。
-- 表结构全部由后端启动时的 GORM AutoMigrate 补齐，本迁移仅负责库级设置。
ALTER DATABASE `akioi` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
