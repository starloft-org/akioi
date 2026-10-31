-- 题库角色：主库、团队库、个人库为预置题库，预置题库不能删除，但允许调整前缀与题号长度。
-- 角色写在独立的列里，题库前缀可以被修改而不影响题目归属的判定。
ALTER TABLE `problem_banks` ADD COLUMN `preset_role` varchar(16) NOT NULL DEFAULT '' AFTER `code_length`;

UPDATE `problem_banks` SET `preset_role` = 'primary' WHERE `prefix` = 'A' AND `preset_role` = '';
UPDATE `problem_banks` SET `preset_role` = 'team' WHERE `prefix` = 'T' AND `preset_role` = '';
UPDATE `problem_banks` SET `preset_role` = 'personal' WHERE `prefix` = 'P' AND `preset_role` = '';
