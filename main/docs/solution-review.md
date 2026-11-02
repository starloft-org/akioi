# 题解审核

用户提交的题解需要管理员审核通过后才会公开，避免题解区出现灌水或错误内容。

## 审核状态

| 状态 | 含义 | 谁能看到 |
| --- | --- | --- |
| `pending` | 待审核 | 作者本人、管理员 |
| `approved` | 已通过 | 所有人 |
| `rejected` | 已拒绝 | 作者本人（能看到驳回说明）、管理员 |

- 普通用户发布题解后状态为 `pending`，发布提示「题解已提交，等待管理员审核」。
- 管理员（含超级管理员）发布题解直接 `approved`，不需要自审。
- 作者修改已被驳回或已通过的题解，会重新回到 `pending`，并清空之前的驳回说明与审核记录；作者可以借此反复修改后重新提交。
- 题目详情页的题解列表只统计与展示 `approved` 的题解（作者自己还能看到自己的待审核 / 被拒题解）。
- 管理员在「题解审核」页面通过或驳回，驳回时可以填写说明（最多 255 字），说明只有作者本人和管理员能看到。

## 权限

后台新增权限键 `solutions`（分组「内容」，名称「题解审核」），用于「题解审核」页面与 `/api/admin/v1/solutions` 接口。超级管理员默认拥有；其他管理员需要在「管理员管理」里勾选。

## 接口

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/admin/v1/solutions` | `solutions` | 分页查询，支持 `page`、`page_size`、`status`、`keyword`、`problem_id` |
| POST | `/api/admin/v1/solutions/:id/review` | `solutions` | 审核，请求体 `{"status":"approved"}` 或 `{"status":"rejected","note":"说明"}` |

管理端查询返回字段：`id`、`problem_id`、`problem_display_id`、`problem_title`、`title`、`content`、`status`、`review_note`、`user_id`、`username`、`nickname`、`visible`、`created_at`、`updated_at`、`reviewed_at`、`reviewed_by`。

## 用户端行为

- 题解列表、题目详情的「题解」区域只显示已通过的题解；作者自己的待审核 / 被拒题解带状态标签（「审核中」/「未通过审核」）。
- 题解详情页顶部会提示审核状态与驳回说明。
- 保存修改后的提示为「已保存，修改后的题解会重新进入审核」。

## 升级说明

新增的 `solutions`、`review_note`、`reviewed_by`、`reviewed_at` 列由 AutoMigrate 自动创建，历史题解的 `status` 默认取列默认值 `approved`，因此升级前已有的题解不会突然消失。
