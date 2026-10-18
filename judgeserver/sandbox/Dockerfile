FROM gcc:13

# 镜像源：默认用阿里云（国内构建快）。海外机器构建时加
#   --build-arg DEBIAN_MIRROR=deb.debian.org
# 即可切回 Debian 官方源，两条路径共用同一份 Dockerfile。
ARG DEBIAN_MIRROR=mirrors.aliyun.com

# 换源同时兼容 sources.list 与 deb822 两种格式
# 判题需要 /usr/bin/time -v 采集内存峰值，基础镜像未包含
RUN set -eux; \
    for f in /etc/apt/sources.list /etc/apt/sources.list.d/debian.sources; do \
      if [ -f "$f" ]; then \
        sed -i -e "s|deb.debian.org|${DEBIAN_MIRROR}|g" \
               -e "s|security.debian.org|${DEBIAN_MIRROR}|g" "$f"; \
      fi; \
    done; \
    apt-get update \
    && apt-get install -y --no-install-recommends time \
    && rm -rf /var/lib/apt/lists/*

# 用户代码编译与运行目录
WORKDIR /work
