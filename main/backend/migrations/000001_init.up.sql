-- 000001 基线：库级字符集设置、题库表与三个默认题库。
-- 业务表结构由后端启动时的 GORM AutoMigrate 补齐，本迁移只负责
-- 库级设置、建题库表（题目表引用它，必须先于 AutoMigrate 存在）与预置题库。

ALTER DATABASE `akioi` CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 题库：题号由「题库前缀 + 定长序号」生成，前缀与题号长度创建后不可修改
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

-- 默认题库：主库给后台创建的公共题目用，团队库/个人库分别给团队题目与个人题目用。
-- 主库先插入，保证它是公共题目的默认题库（id 最小）。
INSERT INTO `problem_banks` (`name`, `prefix`, `code_length`, `remark`, `created_at`, `updated_at`)
SELECT '主库', 'A', 4, '公共题目题库，创建后前缀与题号长度不可修改', NOW(3), NOW(3)
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `problem_banks` WHERE `prefix` = 'A');

INSERT INTO `problem_banks` (`name`, `prefix`, `code_length`, `remark`, `created_at`, `updated_at`)
SELECT '团队库', 'T', 6, '团队题目题库，题目归属团队时自动使用', NOW(3), NOW(3)
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `problem_banks` WHERE `prefix` = 'T');

INSERT INTO `problem_banks` (`name`, `prefix`, `code_length`, `remark`, `created_at`, `updated_at`)
SELECT '个人库', 'P', 6, '个人题目题库，题目归属用户时自动使用', NOW(3), NOW(3)
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `problem_banks` WHERE `prefix` = 'P');
