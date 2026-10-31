-- 000003 回滚：删除个人题单表与 problems 的 owner_id/access，
-- 并移除预置的团队/个人题库（题库下仍有题目时保留，避免题号失去前缀定义）。

DROP TABLE IF EXISTS `user_problem_sets`;

SET @ddl := IF(
  EXISTS(SELECT 1 FROM information_schema.tables
         WHERE table_schema = DATABASE() AND table_name = 'problems')
  AND EXISTS(SELECT 1 FROM information_schema.columns
             WHERE table_schema = DATABASE() AND table_name = 'problems' AND column_name = 'owner_id'),
  'ALTER TABLE `problems` DROP INDEX `idx_problems_owner_id`, DROP COLUMN `owner_id`',
  'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @ddl := IF(
  EXISTS(SELECT 1 FROM information_schema.tables
         WHERE table_schema = DATABASE() AND table_name = 'problems')
  AND EXISTS(SELECT 1 FROM information_schema.columns
             WHERE table_schema = DATABASE() AND table_name = 'problems' AND column_name = 'access'),
  'ALTER TABLE `problems` DROP COLUMN `access`',
  'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

DELETE FROM `problem_banks`
WHERE `prefix` IN ('T', 'P')
  AND NOT EXISTS (SELECT 1 FROM `problems` WHERE `problems`.`bank_id` = `problem_banks`.`id`);
