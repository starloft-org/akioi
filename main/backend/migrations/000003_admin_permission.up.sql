-- 管理员权限：超级管理员可以管理管理员并分配后台权限，权限键以逗号分隔保存。
-- 迁移先于 AutoMigrate 执行，全新库此时连 users 表都还没有，所以先判断表是否存在，
-- 表不存在时什么都不做（users 表由随后的 AutoMigrate 建立，列也会一并建好）。
SET @has_table := (
  SELECT COUNT(*) FROM information_schema.TABLES
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users'
);
SET @has_column := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'admin_permissions'
);
SET @stmt := IF(@has_table > 0 AND @has_column = 0,
  'ALTER TABLE `users` ADD COLUMN `admin_permissions` varchar(255) NOT NULL DEFAULT '''' AFTER `role`',
  'SELECT 1'
);
PREPARE stmt FROM @stmt;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
