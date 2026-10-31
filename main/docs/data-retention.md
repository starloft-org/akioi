# 数据保留与软删除

AKIOI 的删除策略是「能恢复优先」：用户可见的内容一律软删除，记录留在库里、只是从接口与列表里消失；只有功能上必须真删的数据才做硬删除。

## 一、软删除的对象

下列表带 `deleted_at` 列，`DELETE` 只会写入删除时间：

| 表 | 说明 |
| --- | --- |
| `problems` | 题目（含团队题目、个人题目） |
| `contests` | 比赛 |
| `announcements` | 公告 |
| `solutions` | 题解 |
| `discussions` | 讨论 |
| `discussion_comments` | 讨论回复 |
| `teams` | 团队 |
| `team_problem_sets` | 团队题单 |
| `user_problem_sets` | 个人题单 |
| `problem_banks` | 自建题库（预置题库本身禁止删除） |

软删除后：

- 前台与后台的列表、详情、统计都不再包含该记录；
- 比赛被删除后，它的题目关联、报名记录与历史提交仍保留在库中；
- 题目被删除后，题号仍然占用，不会分配给新题目（题号不复用，便于追溯历史提交）；
- 自建题库被删除时，前缀会先被改成 `~<id>`，把前缀释放出来，之后可以重新建同前缀的题库。

## 二、题目测试数据永久保留

删除题目**不会**清理测试数据：

- `testcases` 记录保留在数据库中；
- `TESTDATA_DIR` 下的输入输出文件保留在磁盘上；
- 不会通知评测节点清理缓存。

因此误删题目后，只要把题目恢复回来，测试数据仍然可用。响应里会明确返回 `testdata_retained: true`。

只有显式的「清空测试数据」操作才会真正删除文件与测试点：后台题目测试数据页的「清空」、用户自己的题目测试数据页的「清空」。

## 三、硬删除的例外

下列数据仍然真删，因为软删除会破坏唯一约束或本身没有保留价值：

| 数据 | 原因 |
| --- | --- |
| `testcases` | `(problem_id, case_index)` 唯一索引，软删除会导致重新上传测试点冲突；而且清空测试数据本身就是用户明确要求的操作 |
| `team_members` | `(team_id, user_id)` 唯一索引，软删除会导致退出后无法重新加入 |
| `contest_problems` / `contest_participants` | 纯关系行，只在编辑比赛题目或报名时整体重建 |
| `oauth_bindings` | 第三方绑定，解绑后需要能重新绑定 |
| `submissions` | 提交记录，目前没有删除入口 |
| `system_settings` | 键值配置，按主键覆盖写入 |

## 四、恢复被删除的内容

软删除的数据可以直接在数据库里把 `deleted_at` 清空：

```sql
-- 恢复一个题目
UPDATE problems SET deleted_at = NULL WHERE display_id = 'A0001';

-- 恢复一场比赛
UPDATE contests SET deleted_at = NULL WHERE id = 12;

-- 查看所有被删除的题目
SELECT id, display_id, title, deleted_at FROM problems WHERE deleted_at IS NOT NULL;
```

恢复自建题库时，注意前缀可能已经被释放并被别的题库占用，此时需要先把前缀改成新的值再恢复。

## 五、配合备份

软删除只是第一道保险，仍然建议定期在后台「数据备份」里生成整库备份，详见 [backup.md](backup.md)。
