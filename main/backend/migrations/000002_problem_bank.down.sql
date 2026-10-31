-- 000002 题库回滚：删除归属题库列与题库表。
SET @ddl := IF(
  EXISTS(SELECT 1 FROM information_schema.columns
         WHERE table_schema = DATABASE() AND table_name = 'problems' AND column_name = 'bank_id'),
  'ALTER TABLE `problems` DROP COLUMN `bank_id`',
  'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

DROP TABLE IF EXISTS `problem_banks`;
