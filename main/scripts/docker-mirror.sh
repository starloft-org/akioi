#!/usr/bin/env bash
# 一键配置 Docker 国内镜像加速（腾讯云上海 / 任意国内服务器）
#
# 用法（在服务器上执行，需要 root）：
#   sudo bash docker-mirror.sh
#
# 作用：把 registry-mirrors 合并进 /etc/docker/daemon.json（保留已有其它配置，
#       修改前自动备份），重启 docker，最后实测拉一个 nginx:alpine 看耗时。

set -euo pipefail

DAEMON_JSON="/etc/docker/daemon.json"
# 腾讯云内网镜像放第一位：只在腾讯云 CVM 内网可达，免公网流量、速度最快。
# 后面两个是公网兜底，内网镜像不可用时仍能拉到镜像。
MIRRORS="https://mirror.ccs.tencentyun.com https://docker.m.daocloud.io https://docker.1ms.run"

if [ "$(id -u)" -ne 0 ]; then
  echo "请用 root 执行：sudo bash $0" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "未检测到 docker，请先安装 docker / docker-ce。" >&2
  exit 1
fi

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

# 合并 JSON：保留原有键，只覆盖 registry-mirrors，补一个并发下载数
if command -v python3 >/dev/null 2>&1; then
  DAEMON_JSON="$DAEMON_JSON" MIRRORS="$MIRRORS" python3 - >"$TMP" <<'PY'
import json, os, sys

path = os.environ["DAEMON_JSON"]
mirrors = os.environ["MIRRORS"].split()

data = {}
if os.path.exists(path):
    raw = open(path, encoding="utf-8").read().strip()
    if raw:
        try:
            data = json.loads(raw)
        except json.JSONDecodeError as e:
            print(f"{path} 不是合法 JSON（{e}），为避免覆盖已有配置已中止。", file=sys.stderr)
            sys.exit(2)

data["registry-mirrors"] = mirrors
data.setdefault("max-concurrent-downloads", 10)
print(json.dumps(data, indent=2, ensure_ascii=False))
PY
elif command -v jq >/dev/null 2>&1; then
  mirrors_json="[$(printf '"%s",' $MIRRORS | sed 's/,$//')]"
  if [ -s "$DAEMON_JSON" ]; then
    jq --argjson m "$mirrors_json" \
      '. + {"registry-mirrors": $m, "max-concurrent-downloads": (."max-concurrent-downloads" // 10)}' \
      "$DAEMON_JSON" >"$TMP"
  else
    jq -n --argjson m "$mirrors_json" \
      '{"registry-mirrors": $m, "max-concurrent-downloads": 10}' >"$TMP"
  fi
else
  echo "需要 python3 或 jq 来合并 daemon.json，请先安装其中一个。" >&2
  exit 1
fi

mkdir -p /etc/docker
if [ -s "$DAEMON_JSON" ]; then
  BACKUP="$DAEMON_JSON.bak.$(date +%Y%m%d%H%M%S)"
  cp "$DAEMON_JSON" "$BACKUP"
  echo "已备份原配置到 $BACKUP"
fi
install -m 644 "$TMP" "$DAEMON_JSON"

echo "--- 写入 $DAEMON_JSON ---"
cat "$DAEMON_JSON"

if systemctl is-active --quiet docker 2>/dev/null; then
  echo "--- 重启 docker ---"
  systemctl restart docker
else
  echo "警告：docker 未由 systemd 管理，请手动重启 docker 使配置生效。" >&2
fi

echo "--- 当前生效的镜像加速地址 ---"
docker info --format '{{json .RegistryConfig.Mirrors}}' 2>/dev/null || true

echo "--- 实测拉取 nginx:alpine ---"
START=$(date +%s)
docker pull nginx:alpine
echo "拉取耗时 $(( $(date +%s) - START )) 秒"
