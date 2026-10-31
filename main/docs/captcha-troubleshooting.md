# 人机验证码（StarLoft Captcha）排查指南

对接文档：<https://docs.starloft.cn/captcha/api/v2>

## 常见故障与定位

### 现象一：点登录不弹验证框，接口返回 400

故障链：前端没拿到验证码配置 → 提交时未携带票据 → 后端校验拒绝。

按顺序检查：

1. **前端镜像是否已重建**。验证码逻辑在前端代码里，后端开了开关但前端还是旧镜像时，登录请求不带票据，直接 400。
2. 打开浏览器开发者工具看 `GET /api/v1/site/captcha?scene=account` 的返回：
   - `{"enabled": false}`：后台开关没开，或后端拿不到 API 密钥（检查后台「系统设置 → 密钥与集成」里的验证码 API Key / API Secret，留空时回退短信 API Key / API Secret）。
   - `503`：后端已开启但验证码平台调用失败，看 `akioi_backend` 日志中「拉取验证码配置失败」一行的具体错误。
   - 正常返回 `{"enabled": true, "captcha_app_id": "...", "web_js_url": "..."}` 但仍不弹框：检查 `web_js_url` 是否被浏览器拦截（广告插件/网络不通）。

### 现象二：后端日志出现「拉取验证码配置失败」

平台接口调用失败，按下面步骤排查上游：

1. **密钥权限**（最常见）：平台要求密钥权限包含 `captcha_config` 与 `captcha_verify`（或 `all`）。短信密钥若只开通了短信接口，回退使用时会 403。到 StarLoft 控制台检查 API 密钥权限，或单独申请验证码密钥填入后台「系统设置 → 密钥与集成」的验证码 API Key / API Secret。
2. **实名要求**：账户需完成个人实名（`realname_status ≥ 1`）。
3. **额度不足**：接口返回 402 表示资源包与余额均不足，充值后恢复。
4. **签名头缺失**：平台强制要求 5 个鉴权头（`X-Api-Key`、`X-Sign`、`X-Sign-Version: hmac_sha256_v2`、`X-Timestamp`、`X-Nonce`），缺失任何都会返回 400 "unsupported sign version"。

### 平台结果码速查

| captcha_code | 含义 | 处理 |
| --- | --- | --- |
| 1 | 验证通过 | 放行 |
| 7 | 答案或 randstr 不匹配 | 重新验证 |
| 8 | 票据过期（5 分钟） | 重新验证 |
| 9 | 票据被重复校验 | 一次票据只能用一次 |
| 13 | IP 不匹配 | 校验传入的 `ip` 是否为用户真实外网 IP |
| 15 | 票据解密失败 | ticket/randstr 需来自同一次验证 |
| 16 | 验证 ID 不匹配 | config 与 verify 的三组合参数必须一致 |
| 21 | 前端容灾票据 | 按业务风险策略决定 |

## 手动诊断命令

在能访问外网的机器上执行（密钥填平台 API Key）：

```bash
API_KEY="填入密钥"
API_SECRET="填入密钥对应的 Secret"
METHOD="GET"
PATH_="/v2/captcha/config"
QUERY="client_type=web&scene=account&verify_type=slide"
TS=$(date +%s)
NONCE=$(openssl rand -hex 16)
BODY_HASH=$(printf '%s' "" | openssl dgst -sha256 -hex | awk '{print $2}')
CANONICAL=$(printf '%s\n%s\n%s\n%s\n%s\n%s' "$METHOD" "$PATH_" "$QUERY" "$TS" "$NONCE" "$BODY_HASH")
SIGN=$(printf '%s' "$CANONICAL" | openssl dgst -sha256 -hmac "$API_SECRET" -hex | awk '{print $2}')

curl -s "https://api.starloft.cn${PATH_}?${QUERY}" \
  -H "X-Api-Key: ${API_KEY}" \
  -H "X-Sign: ${SIGN}" \
  -H "X-Sign-Version: hmac_sha256_v2" \
  -H "X-Timestamp: ${TS}" \
  -H "X-Nonce: ${NONCE}"
```

- 返回 `{"code":0,...}`：平台侧正常，问题在 akioi 配置（检查后台「密钥与集成」与验证码开关）。
- 返回 `403`：密钥权限不含 `captcha_config`。
- 返回 `401`：签名或时间戳不对（检查服务器时间是否准确）。
- 返回 `400 unsupported sign version`：请求头缺失（`X-Sign-Version` 等五个头必须齐全）。

## 计费说明

- 仅平台确认收费（`charge_status=1`）时计 1 次；网络异常或接口报错不计费。
- 校验未通过但平台已收费的仍计 1 次。
- 票据 5 分钟有效、只能校验一次，前端每次弹框都会产生新票据。
