-- 管理员权限：超级管理员可以管理管理员并分配后台权限，权限键以逗号分隔保存。
-- 迁移先于 AutoMigrate 执行，全新库此时还没有 users 表，因此用 information_schema 守卫。
SET @has_column := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'admin_permissions'
);
SET @stmt := IF(@has_column = 0,
  'ALTER TABLE `users` ADD COLUMN `admin_permissions` varchar(255) NOT NULL DEFAULT '''' AFTER `role`',
  'SELECT 1'
);
PREPARE stmt FROM @stmt;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
