# 配置说明：.env 与后台设置的分工

主站（`main`）的配置分成两部分：**启动前必须提供**的连接信息仍放在 `.env`，**运行期可调整**的业务设置与第三方密钥保存在数据库，通过后台「系统设置」修改，保存后立即生效、无需重启。

评测节点（`judgeserver`）不参与这套机制，它的 `.env` 仍然包含数据库、Redis、主站地址与同步密钥等全部配置。

## 一、.env 保留的键

这些键在容器启动、连接数据库之前就要用到，因此只能写在 `main/.env`：

| 键 | 说明 |
| --- | --- |
| `APP_ENV` | 运行环境标识 |
| `SERVER_ADDR` | 后端监听地址 |
| `FRONTEND_PORT` | 前端对外端口（默认 `3000`，HTTP；HTTPS 由外层反向代理终止） |
| `DB_HOST` / `DB_PORT` / `DB_BIND_ADDR` | MySQL 连接信息 |
| `DB_USER` / `DB_PASSWORD` | MySQL 账号（`DB_PASSWORD` 必填） |
| `DB_MAX_OPEN_CONNS` / `DB_MAX_IDLE_CONNS` | 连接池大小 |
| `REDIS_ADDR` / `REDIS_BIND_ADDR` / `REDIS_PASSWORD` / `REDIS_DB` | Redis 连接信息（`REDIS_ADDR` 必填） |
| `ADMIN_INIT_USERNAME` / `ADMIN_INIT_PASSWORD` | 首次启动创建管理员用 |
| `TESTDATA_DIR` | 测试数据目录 |
| `JUDGE_SYNC_SECRET` | 仅在库中没有该密钥时被导入一次（见下文） |
| `LOG_DIR` / `LOG_MAX_MB` / `UPLOAD_DIR` | 日志与上传目录 |
| `BACKUP_DIR` | 数据库备份目录（默认 `/app/backups`，对应 `data/backups`） |

`ADMIN_INIT_USERNAME` / `ADMIN_INIT_PASSWORD` 只在数据库里还没有任何管理员时使用：已经有管理员账号后再改这两个键不会生效，改密码请用 `akioi-cli reset-password`。

生成这份 `.env` 可以执行仓库内的脚本：

```bash
cd /akioi && git pull --ff-only && bash main/scripts/init-env.sh
```

脚本会随机生成数据库、Redis 与管理员密码，并把两端必须一致的 `JUDGE_SYNC_SECRET` 同时写入主站与评测节点的 `.env`。

## 二、数据库中的配置

登录后台「系统设置」可以修改下列内容，它们保存在 `system_settings` 表的两行里：`runtime` 行放业务开关与限制，`secrets` 行放密钥，两者分开存储，密钥不会随业务设置一起返回给前端。

- 站点与登录：站点名、主题色、登录有效期、注册开关、评论开关
- 人机验证码、邮件、短信的启用开关与模板
- 访问与上传限制、评测节点地址与同步超时
- 「密钥与集成」面板：公众号 AppID/AppSecret、短信 API Key/Secret、SMTP 服务器与账号、验证码 API Key/Secret、评测节点同步密钥、登录密钥

密钥类字段保存后不会回显，输入框留空表示保持原值；需要更换就填入新值，需要清空就勾选下面对应的「清除」。

## 三、首次启动的导入规则

1. 若 `system_settings` 中没有 `secrets` 行，服务会把 `.env` 里残留的旧键（`JWT_SECRET`、`WECHAT_*`、`SMS_*`、`SMTP_*`、`CAPTCHA_*`、`JUDGE_SYNC_SECRET`）导入一次并写库。
2. 导入之后只认数据库：改 `.env` 不会再影响密钥，清空密钥也不会被 `.env` 复活。
3. 登录密钥（`JWT_SECRET`）导入为空时会自动生成一个随机值并入库，所以 `.env` 里可以完全不写它。

## 四、评测节点同步密钥

`JUDGE_SYNC_SECRET` 用于主站与评测节点之间的回传鉴权：主站把密钥存在数据库，评测节点仍从自己的 `.env` 读取。两者必须完全一致，否则回传会被拒绝（`同步密钥不正确`）。

- 在后台「密钥与集成」修改同步密钥后，需要同步修改每个评测节点 `.env` 中的 `JUDGE_SYNC_SECRET` 并重启节点。
- 修改前已导入库的值不会被 `.env` 覆盖，这是为了防止管理员清空密钥后又被环境变量恢复。

## 五、重新生成登录密钥

「密钥与集成」里的「重新生成」会换掉签发 JWT 的密钥，保存后所有已登录用户都会被登出、需要重新登录。用于怀疑密钥泄漏时轮换，日常不需要操作。

## 六、常见问题

| 现象 | 原因与处理 |
| --- | --- |
| 登录后立刻掉线 | 登录密钥被重新生成，重新登录即可 |
| 验证码面板不弹框 | 后台没有启用验证码，或「密钥与集成」里没有验证码/短信密钥 |
| 收不到邮件 | 邮件开关没开，或 SMTP 服务器、发件人、密码未填全 |
| 短信发送失败 | 短信开关没开、模板 ID 未填、或 API Key 权限不含短信接口 |
| 评测结果一直不回来 | 主站与节点的 `JUDGE_SYNC_SECRET` 不一致，或节点地址配置错误 |
| 改了 `.env` 不生效 | 密钥类配置以数据库为准，请在后台修改；`.env` 只在库中缺少 `secrets` 行时被导入一次 |

从旧版本升级时，如果 `.env` 里仍写着旧的密钥键，首次启动会自动导入，导入成功后可以删掉这些键；也可以直接把密钥填到后台「密钥与集成」。
