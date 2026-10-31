-- 000002 题库：题号由「题库前缀 + 定长序号」生成，problems 增加归属题库。
-- 表结构与 GORM 模型保持一致，列已存在时重复执行不会报错。

CREATE TABLE IF NOT EXISTS `problem_banks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(64) NOT NULL,
  `prefix` varchar(8) NOT NULL,
  `code_length` bigint NOT NULL DEFAULT 4,
  `remark` varchar(255) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_problem_banks_prefix` (`prefix`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 首次升级时补一个默认题库，存量题号 A0000 一类继续可用。
INSERT INTO `problem_banks` (`name`, `prefix`, `code_length`, `remark`, `created_at`, `updated_at`)
SELECT '默认题库', 'A', 4, '系统初始化生成的默认题库', NOW(3), NOW(3)
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `problem_banks`);

-- problems 表此时可能尚未由 AutoMigrate 创建，仅当表存在且缺少 bank_id 时补列。
SET @ddl := IF(
  EXISTS(SELECT 1 FROM information_schema.tables
         WHERE table_schema = DATABASE() AND table_name = 'problems')
  AND NOT EXISTS(SELECT 1 FROM information_schema.columns
                 WHERE table_schema = DATABASE() AND table_name = 'problems' AND column_name = 'bank_id'),
  'ALTER TABLE `problems` ADD COLUMN `bank_id` bigint unsigned NOT NULL DEFAULT 0, ADD INDEX `idx_problems_bank_id` (`bank_id`)',
  'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 存量题目统一归入默认题库（id 最小的题库）。
SET @backfill := IF(
  EXISTS(SELECT 1 FROM information_schema.columns
         WHERE table_schema = DATABASE() AND table_name = 'problems' AND column_name = 'bank_id'),
  'UPDATE `problems` SET `bank_id` = (SELECT MIN(`id`) FROM `problem_banks`) WHERE `bank_id` = 0',
  'SELECT 1');
PREPARE stmt FROM @backfill;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
