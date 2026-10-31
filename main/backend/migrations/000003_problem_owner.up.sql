-- 000003 题目归属与访问权限：problems 增加 owner_id/access，新增个人题单表，
-- 并预置「团队题目 T」「个人题目 P」两个题库（题号长度 6）。
-- 表结构与 GORM 模型保持一致，列已存在时重复执行不会报错。

-- 个人题目创建者与访问权限
SET @ddl := IF(
  EXISTS(SELECT 1 FROM information_schema.tables
         WHERE table_schema = DATABASE() AND table_name = 'problems')
  AND NOT EXISTS(SELECT 1 FROM information_schema.columns
                 WHERE table_schema = DATABASE() AND table_name = 'problems' AND column_name = 'owner_id'),
  'ALTER TABLE `problems` ADD COLUMN `owner_id` bigint unsigned NOT NULL DEFAULT 0, ADD INDEX `idx_problems_owner_id` (`owner_id`)',
  'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @ddl := IF(
  EXISTS(SELECT 1 FROM information_schema.tables
         WHERE table_schema = DATABASE() AND table_name = 'problems')
  AND NOT EXISTS(SELECT 1 FROM information_schema.columns
                 WHERE table_schema = DATABASE() AND table_name = 'problems' AND column_name = 'access'),
  'ALTER TABLE `problems` ADD COLUMN `access` bigint NOT NULL DEFAULT 1',
  'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 个人题单
CREATE TABLE IF NOT EXISTS `user_problem_sets` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `owner_id` bigint unsigned NOT NULL,
  `title` varchar(128) NOT NULL,
  `description` varchar(255) DEFAULT NULL,
  `problem_ids` text,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_problem_sets_owner_id` (`owner_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 预置团队题库与个人题库：团队题号 T 开头、个人题号 P 开头，均为 6 位序号。
INSERT INTO `problem_banks` (`name`, `prefix`, `code_length`, `remark`, `created_at`, `updated_at`)
SELECT '团队题目', 'T', 6, '团队题目的题号前缀，创建后不可修改', NOW(3), NOW(3)
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `problem_banks` WHERE `prefix` = 'T');

INSERT INTO `problem_banks` (`name`, `prefix`, `code_length`, `remark`, `created_at`, `updated_at`)
SELECT '个人题目', 'P', 6, '个人题目的题号前缀，创建后不可修改', NOW(3), NOW(3)
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `problem_banks` WHERE `prefix` = 'P');
