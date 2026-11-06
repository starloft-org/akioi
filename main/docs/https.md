# HTTPS 与 Let's Encrypt 自动证书

主站前端（`main/frontend`）现在默认只通过 HTTPS 提供业务：80 端口不再承载任何页面或接口，只保留两件事——

1. 响应 Let's Encrypt 的 HTTP-01 证书校验（`/.well-known/acme-challenge/`）；
2. 其余请求返回 `301` 跳到 `https://` 同地址。

证书由前端容器自己申请和续期，不需要额外的 certbot 容器，也不需要宿主机装 nginx。

## 一、配置

在 `main/.env` 中填写：

```dotenv
DOMAIN=oj.example.com      # 站点域名，必须已经解析到这台服务器
ACME_EMAIL=admin@example.com  # 证书到期通知邮箱
```

其余可选键：

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `FRONTEND_PORT` | `80` | 80 端口映射，只用于 ACME 校验与跳转；自动申请要求公网能访问 80 |
| `CERT_NAME` | `akioi` | 证书目录名，对应 `data/letsencrypt/live/<CERT_NAME>`；同机部署多个站点时各自不同。测试环境会自动加上 `-staging` 后缀 |
| `LETSENCRYPT_STAGING` | `0` | 设为 `1` 时使用 Let's Encrypt 测试环境，调试时不会消耗正式环境的频率额度 |

改完执行：

```bash
docker compose -f main/docker-compose.yml up -d --build frontend
```

## 二、启动时发生了什么

1. 容器入口脚本检查 `data/letsencrypt/live/<CERT_NAME>/`：
   - 有正式证书 → 把 `/etc/nginx/certs` 的软链接指向它；
   - 没有 → 生成一张 30 天的临时自签名证书，保证 nginx 能先启动（浏览器会提示不受信任）。
2. 启动 nginx，80 端口开始响应 ACME 校验文件。
3. 如果配置了 `DOMAIN` 和 `ACME_EMAIL` 且还没有正式证书，立即用 webroot 方式申请。
4. 申请成功后把软链接切到正式证书并 `nginx -s reload`，浏览器随即变成受信任的 HTTPS。
5. 后台每 12 小时执行一次 `certbot renew`，续期成功自动 reload，无需人工介入。

申请失败不会让容器退出，会继续用自签名证书服务，并在 12 小时后重试。失败原因看日志：

```bash
docker compose -f main/docker-compose.yml logs -f frontend
```

## 三、验证

```bash
# 80 应该只返回 301（或 ACME 校验文件）
curl -I http://oj.example.com/

# 证书签发者应为 Let's Encrypt，且域名匹配
echo | openssl s_client -connect oj.example.com:443 -servername oj.example.com 2>/dev/null \
  | openssl x509 -noout -issuer -subject -dates
```

看到 `issuer=... Let's Encrypt ...` 即表示正式证书已生效。

## 四、常见问题

| 现象 | 处理 |
| --- | --- |
| 日志里 certbot 报 `Connection refused` / `Timeout` | 域名没解析到本机，或公网访问不到 80 端口；检查 DNS、防火墙与安全组 |
| 报 `too many failed authorizations` / 频率限制 | 先用 `LETSENCRYPT_STAGING=1` 调试，确认流程通后再改回 `0` 重建 |
| 站点仍是自签名证书 | 看前端日志里 certbot 的报错；修好后 `docker compose ... restart frontend` 会立即重试，不必等 12 小时 |
| 改了 `DOMAIN` 但证书还是旧的 | 证书目录名没变时会沿用旧证书。换域名建议同时改 `CERT_NAME`，或清空 `data/letsencrypt` 后重建前端 |
| 需要强制重新申请 | 删除 `data/letsencrypt/live/<CERT_NAME>`（或整个 `data/letsencrypt`）后重启前端；注意 Let's Encrypt 每周签发次数有限 |
| 浏览器仍跳 http | 清理浏览器 HSTS 缓存；确认访问的是域名而不是 IP |

## 五、评测节点要跟着改

启用 HTTPS 后 80 只做 301 跳转，评测节点如果继续用 `http://` 回传，POST 会被重定向成 GET 而失败。请把 `judgeserver/.env` 里的 `MAIN_BASE_URL` 改成域名形式：

```dotenv
MAIN_BASE_URL=https://oj.example.com
```

`main/scripts/init-env.sh` 在检测到 `main/.env` 里有 `DOMAIN` 时会自动按 `https://<DOMAIN>` 生成这个值。评测节点使用的是正式证书，Go 的 HTTP 客户端可以正常校验。

## 六、安全说明

- 443 上启用 TLS 1.2/1.3，关闭会话票据，开启会话缓存；正式证书生效后返回 `Strict-Transport-Security: max-age=31536000`。
- 80 端口不返回任何业务内容，只有 ACME 校验与 301。
- 证书与续期状态保存在 `data/letsencrypt`，请一并纳入备份；`data/certbot-webroot` 只是校验文件的临时目录，可以不备份。
- 证书私钥权限由 certbot 控制（`live` 目录下为 600），容器内只有 root 可读。
