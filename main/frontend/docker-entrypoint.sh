#!/bin/sh
# 前端容器入口：维护 HTTPS 证书，让 nginx 始终有证书可用。
#
# 流程：
#   1. 有 Let's Encrypt 证书就把 /etc/nginx/certs 指向它，否则生成临时自签名证书兜底；
#   2. 启动 nginx（80 端口此时可用于 ACME 校验）；
#   3. 配置了 DOMAIN 与 ACME_EMAIL 且还没有正式证书时，自动申请；
#   4. 后台每 12 小时续期一次，续期成功自动 reload nginx。
#
# 可用环境变量：
#   DOMAIN                站点域名，如 oj.example.com
#   ACME_EMAIL            证书到期通知邮箱
#   CERT_NAME             证书存放名，默认 akioi（对应 /etc/letsencrypt/live/akioi）
#   LETSENCRYPT_STAGING   设为 1 时使用 Let's Encrypt 测试环境，避免正式环境频率限制
set -eu

DOMAIN="${DOMAIN:-}"
ACME_EMAIL="${ACME_EMAIL:-${LETSENCRYPT_EMAIL:-}}"
CERT_NAME="${CERT_NAME:-akioi}"
STAGING="${LETSENCRYPT_STAGING:-0}"
if [ "$STAGING" = "1" ]; then
    # 测试环境的证书单独存一份，切回正式环境时不会误用测试证书
    CERT_NAME="${CERT_NAME}-staging"
fi
WEBROOT=/var/www/certbot
LE_DIR="/etc/letsencrypt/live/$CERT_NAME"
CERT_DIR=/etc/nginx/certs

log() { echo "[entrypoint] $*"; }

mkdir -p "$WEBROOT" "$CERT_DIR"

has_le_cert() {
    [ -s "$LE_DIR/fullchain.pem" ] && [ -s "$LE_DIR/privkey.pem" ]
}

cert_matches_domain() {
    [ -n "$DOMAIN" ] || return 1
    openssl x509 -in "$LE_DIR/cert.pem" -noout -checkhost "$DOMAIN" >/dev/null 2>&1
}

prepare_cert_links() {
    if has_le_cert; then
        ln -sfn "$LE_DIR/fullchain.pem" "$CERT_DIR/fullchain.pem"
        ln -sfn "$LE_DIR/privkey.pem" "$CERT_DIR/privkey.pem"
        return 0
    fi

    # 自签名证书只用于让 nginx 能先跑起来，浏览器会提示不受信任
    if [ ! -s "$CERT_DIR/selfsigned.crt" ] ||
        ! openssl x509 -checkend 604800 -noout -in "$CERT_DIR/selfsigned.crt" >/dev/null 2>&1; then
        log "没有可用证书，生成临时自签名证书（CN=${DOMAIN:-localhost}）"
        openssl req -x509 -nodes -newkey rsa:2048 -days 30 \
            -subj "/CN=${DOMAIN:-localhost}" \
            -keyout "$CERT_DIR/selfsigned.key" \
            -out "$CERT_DIR/selfsigned.crt" >/dev/null 2>&1
    fi
    ln -sfn "$CERT_DIR/selfsigned.crt" "$CERT_DIR/fullchain.pem"
    ln -sfn "$CERT_DIR/selfsigned.key" "$CERT_DIR/privkey.pem"
}

issue_cert() {
    if has_le_cert; then
        if cert_matches_domain; then
            return 0
        fi
        log "现有证书不包含域名 $DOMAIN，跳过自动申请；如需换域名请清理 data/letsencrypt 或修改 CERT_NAME"
        return 1
    fi
    if [ -z "$DOMAIN" ] || [ -z "$ACME_EMAIL" ]; then
        log "DOMAIN / ACME_EMAIL 未配置，跳过 Let's Encrypt 申请，继续使用自签名证书"
        return 1
    fi

    set -- --webroot -w "$WEBROOT" -d "$DOMAIN" --cert-name "$CERT_NAME" \
        --email "$ACME_EMAIL" --agree-tos --non-interactive
    if [ "$STAGING" = "1" ]; then
        set -- "$@" --staging
    fi

    log "向 Let's Encrypt 申请证书：$DOMAIN（首次申请需要 80 端口可被公网访问、域名已解析）"
    if certbot certonly "$@"; then
        log "证书申请成功：$LE_DIR"
        return 0
    fi
    log "证书申请失败，12 小时后自动重试；在此期间继续使用自签名证书"
    return 1
}

renew_once() {
    # 续期成功后由 deploy-hook 重载 nginx
    certbot renew --webroot -w "$WEBROOT" --quiet \
        --deploy-hook "nginx -s reload" || true
}

renew_loop() {
    while :; do
        sleep 12h
        if has_le_cert; then
            renew_once
        elif issue_cert; then
            prepare_cert_links
            nginx -s reload || true
        fi
    done
}

prepare_cert_links

nginx -g 'daemon off;' &
NGINX_PID=$!
trap 'kill -TERM "$NGINX_PID" 2>/dev/null || true; wait "$NGINX_PID" 2>/dev/null || true' TERM INT

# 等 nginx 监听 80 后再做首次申请，否则 ACME 校验拿不到 challenge 文件
sleep 3
if has_le_cert; then
    # 停机较久时证书可能已过期，启动时先补一次续期
    renew_once
else
    if issue_cert; then
        prepare_cert_links
        nginx -s reload || true
    fi
fi

renew_loop &
wait "$NGINX_PID"
