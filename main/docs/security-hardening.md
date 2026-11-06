# 安全加固说明

本文件记录 `SECURITY_AUDIT.md` 中 S01–S13 的处理方式，便于部署与后续维护时核对。

## 评测节点（judgeserver）

| 编号 | 问题 | 处理 |
| --- | --- | --- |
| S01 | 评测进程通过符号链接读取沙箱外文件 | 新增 `internal/judge/safeio.go`：读取时间报告、stderr 等临时文件前先用 `O_NOFOLLOW` 打开并核验 `IsRegular`，只读取普通文件，超长内容直接拒绝 |
| S03 | 整份测试数据目录挂进选手沙箱 | 每个测试点单独准备输入目录 `sandbox_inputs/<submission>/<index>`，只复制当前点的 `.in`（SPJ 额外复制 `.out`），沙箱只挂这一个目录 |
| S04 | 浮点比较接受 NaN/Inf | `Compare` 对 `eps` 做有限性检查，`compareFloat` 遇到 NaN/Inf 直接判错 |
| S05 | 节点解压 zip 无限制 | 限制文件数与解压后总大小，超出即拒绝 |
| S06 | 工作目录无配额、stderr 无上限 | 每道题执行后检查工作目录大小，超过 `WORK_QUOTA_MB`（默认 512）终止；stderr 只保留前 64KB；docker 追加 `--security-opt no-new-privileges`、`--ulimit fsize` |

## 主站（backend）

| 编号 | 问题 | 处理 |
| --- | --- | --- |
| S02 | 拥有「用户管理」权限的管理员可改超管密码/状态 | `AdminUpdateUser` 拒绝修改超级管理员账号（本人除外）；非超管也不能修改其他管理员的状态、密码与昵称 |
| S05 | 主站解压 zip 无限制 | `ExtractZip` 限制文件数 2 万、单文件 256MB、总解压 2GB |
| S07 | 用户自建题的自定义积分进入全站排名 | 全站积分只统计公共题库里的公共题（`owner_id = 0 AND team_id = 0 AND access = 公开`），且进行中的比赛提交不计入，比赛结束后才计入 |
| S09 | 提交与用户主页缺少题目访问控制 | 新增 `visibleProblemIDs`：只包含公共题、自己的个人题、所在团队的团队题；提交列表、提交详情、提交状态、用户主页最近提交都按它过滤，隐藏题与其他人的私有题不可见 |
| S10 | 无条件信任 `X-Forwarded-For` | 真实 IP 交给 gin 的 `SetTrustedProxies` 判断，只有受信任代理（`TRUSTED_PROXIES`）的转发头才生效；限流除按 IP 外再按会话令牌计数，伪造来源 IP 无法放大单账号请求量 |
| S11 | SMTP 可降级明文认证 | 要求 STARTTLS（不支持直接报错），登录凭据只在加密连接上发送，连接带 10 秒超时，TLS 最低 1.2 |
| S12 | 无「用户管理」权限的管理员能读完整手机号 | `GET /api/admin/v1/users` 改为需要「用户管理」权限；其他管理员改用 `GET /api/admin/v1/users/selector`（只有 id、用户名、昵称）；列表里没有权限的管理员看到的手机号中间四位被隐藏 |
| S13 | 请求体在解析后才校验大小 | 新增 `middleware.BodyLimit()`：普通 JSON 接口 2MB、测试数据上传 512MB、题目导入 1GB，在读取请求体之前套 `MaxBytesReader` |
| S08 | 默认 HTTP 部署 | 见下方「部署要求」，生产环境必须在反向代理层终止 TLS |

## 部署要求（S08）

1. 在服务器上为站点配置 HTTPS（证书可放在 nginx 或云厂商负载均衡上），并强制跳转：

   ```nginx
   server {
       listen 80;
       server_name oj.example.com;
       return 301 https://$host$request_uri;
   }

   server {
       listen 443 ssl http2;
       server_name oj.example.com;
       ssl_certificate     /etc/nginx/certs/fullchain.pem;
       ssl_certificate_key /etc/nginx/certs/privkey.pem;
       add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
       location / {
           proxy_pass http://frontend:80;
           proxy_set_header Host $host;
           proxy_set_header X-Real-IP $remote_addr;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_set_header X-Forwarded-Proto https;
       }
   }
   ```

2. 如果 TLS 由前置代理终止、后端只接受内网请求，可以在 `.env` 里限制受信任代理：

   ```
   TRUSTED_PROXIES=127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
   ```

   只有这些地址发来的 `X-Forwarded-For` 才会被当成真实 IP，其他请求一律使用直连地址。

3. 评测节点不需要公网暴露，`judgeserver` 只需能与主站通信即可。
