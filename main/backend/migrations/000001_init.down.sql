-- 000001 基线回滚：库级字符集设置无需回滚，只移除预置题库表。
DROP TABLE IF EXISTS `problem_banks`;
