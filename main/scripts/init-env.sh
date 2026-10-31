#!/usr/bin/env bash
# 一键生成 main 与 judgeserver 的 .env（单服务器同时部署主站与评测节点）
#
# 用法（在仓库根目录执行）：
#   bash main/scripts/init-env.sh            # 自动取本机内网 IP 作为主站地址
#   bash main/scripts/init-env.sh 10.0.0.9   # 显式指定主站地址（内网 IP 或域名）
#
# 行为：
#   - 缺少 .env 时从 .env.example 复制，已有 .env 只补空项，不覆盖已填写的值
#   - DB_PASSWORD / REDIS_PASSWORD / JWT_SECRET / ADMIN_INIT_PASSWORD 各自随机生成
#   - JUDGE_SYNC_SECRET 两端写成同一个值（沿用已有值，没有才生成）
#   - judgeserver 的 MAIN_BASE_URL 按主站地址与 FRONTEND_PORT 生成
#   - judgeserver 的 HOST_TESTDATA_DIR / HOST_WORK_DIR 按仓库路径生成并建好目录

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

MAIN_ENV="main/.env"
JUDGE_ENV="judgeserver/.env"

if [ ! -f main/.env.example ] || [ ! -f judgeserver/.env.example ]; then
  echo "未找到 main/.env.example 或 judgeserver/.env.example，请确认在仓库根目录执行" >&2
  exit 1
fi

gen() { openssl rand -hex "${1:-16}"; }

val() { grep -m1 "^$2=" "$1" 2>/dev/null | cut -d= -f2- | tr -d '\r' || true; }

setval() {
  local file="$1" key="$2" value="$3"
  if grep -q "^$key=" "$file"; then
    sed -i "s|^$key=.*$|$key=$value|" "$file"
  else
    printf '%s=%s\n' "$key" "$value" >>"$file"
  fi
}

fill() {
  [ -n "$(val "$1" "$2")" ] || setval "$1" "$2" "$3"
}

[ -f "$MAIN_ENV" ] || cp main/.env.example "$MAIN_ENV"
[ -f "$JUDGE_ENV" ] || cp judgeserver/.env.example "$JUDGE_ENV"

fill "$MAIN_ENV" DB_PASSWORD "$(gen 16)"
fill "$MAIN_ENV" REDIS_PASSWORD "$(gen 16)"
fill "$MAIN_ENV" JWT_SECRET "$(gen 32)"
fill "$MAIN_ENV" ADMIN_INIT_PASSWORD "$(gen 12)"
fill "$JUDGE_ENV" DB_PASSWORD "$(gen 16)"
fill "$JUDGE_ENV" REDIS_PASSWORD "$(gen 16)"

# 两端必须一致：已有值优先，否则生成一次后同步写入两边
SYNC="$(val "$MAIN_ENV" JUDGE_SYNC_SECRET)"
[ -n "$SYNC" ] || SYNC="$(val "$JUDGE_ENV" JUDGE_SYNC_SECRET)"
[ -n "$SYNC" ] || SYNC="$(gen 24)"
setval "$MAIN_ENV" JUDGE_SYNC_SECRET "$SYNC"
setval "$JUDGE_ENV" JUDGE_SYNC_SECRET "$SYNC"

HOST="${1:-}"
EXPLICIT=0
[ -n "$HOST" ] && EXPLICIT=1

if [ "$EXPLICIT" -eq 0 ]; then
  HOST="$(ip route get 1.1.1.1 2>/dev/null | awk '{print $7; exit}' || true)"
fi
if [ -z "$HOST" ]; then
  HOST="$(hostname -I 2>/dev/null | awk '{print $1}' | tr -d '\r' || true)"
fi

if [ -n "$HOST" ]; then
  PORT="$(val "$MAIN_ENV" FRONTEND_PORT)"
  [ -n "$PORT" ] || PORT=80
  if [ "$PORT" = "80" ]; then
    BASE="http://$HOST"
  else
    BASE="http://$HOST:$PORT"
  fi
  if [ "$EXPLICIT" -eq 1 ]; then
    setval "$JUDGE_ENV" MAIN_BASE_URL "$BASE"
  else
    fill "$JUDGE_ENV" MAIN_BASE_URL "$BASE"
  fi
else
  echo "警告：未取到本机内网 IP，请重跑并显式指定，例如：bash main/scripts/init-env.sh 10.0.0.9" >&2
fi

fill "$JUDGE_ENV" HOST_TESTDATA_DIR "$ROOT/judgeserver/data/testdata"
fill "$JUDGE_ENV" HOST_WORK_DIR "$ROOT/judgeserver/data/work"
mkdir -p judgeserver/data/testdata judgeserver/data/work
chmod 600 "$MAIN_ENV" "$JUDGE_ENV"

echo
echo "===== $MAIN_ENV"
for key in DB_PASSWORD REDIS_PASSWORD JWT_SECRET ADMIN_INIT_USERNAME ADMIN_INIT_PASSWORD JUDGE_SYNC_SECRET FRONTEND_PORT; do
  printf '  %-20s %s\n' "$key" "$(val "$MAIN_ENV" "$key")"
done
echo "===== $JUDGE_ENV"
for key in DB_PASSWORD REDIS_PASSWORD MAIN_BASE_URL JUDGE_SYNC_SECRET HOST_TESTDATA_DIR HOST_WORK_DIR; do
  printf '  %-20s %s\n' "$key" "$(val "$JUDGE_ENV" "$key")"
done
echo
echo "下一步："
echo "  docker compose -f main/docker-compose.yml up -d --build"
echo "  docker compose -f judgeserver/docker-compose.yml up -d --build"
echo "后台「系统设置 → 评测节点」填 http://<主站地址>:9000"
echo "admin 密码忘记时：docker compose -f main/docker-compose.yml exec backend /app/akioi-cli reset-password -user admin -password '新密码'"
