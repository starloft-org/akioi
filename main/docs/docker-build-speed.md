# 构建提速：Docker 镜像加速

首次 `docker compose up -d --build` 慢，几乎全部时间都花在从 Docker Hub 拉基础镜像上。
实测日志里 `nginx:alpine`（20.5MB）一项就用了 142 秒，Go 编译反而只占 17.9 秒。

## 一键配置（推荐）

在服务器上执行：

```bash
sudo bash main/scripts/docker-mirror.sh
```

脚本做的事：

1. 把 `registry-mirrors` 合并进 `/etc/docker/daemon.json`（保留已有其它键，改前自动备份）；
2. 重启 docker；
3. 打印当前生效的加速地址，并实测拉一次 `nginx:alpine` 给出耗时。

内置的加速源顺序：

| 顺序 | 地址 | 说明 |
| --- | --- | --- |
| 1 | `https://mirror.ccs.tencentyun.com` | 腾讯云 CVM **内网**镜像，免公网流量，速度最快 |
| 2 | `https://docker.m.daocloud.io` | 公网兜底 |
| 3 | `https://docker.1ms.run` | 公网兜底 |

**注意**：`mirror.ccs.tencentyun.com` 只能从腾讯云服务器内网访问，公网或其它云机器上解析不到。
Docker 会按顺序尝试，内网源不可用时自动落到后面的公网源，所以同一份配置在内网机和外网机上都能用。

## 手动配置

不想跑脚本时，直接写 `/etc/docker/daemon.json`：

```json
{
  "registry-mirrors": [
    "https://mirror.ccs.tencentyun.com",
    "https://docker.m.daocloud.io",
    "https://docker.1ms.run"
  ],
  "max-concurrent-downloads": 10
}
```

然后：

```bash
sudo systemctl daemon-reload && sudo systemctl restart docker
docker info | grep -A4 "Registry Mirrors"    # 确认已生效
docker pull nginx:alpine                      # 应该几秒内完成
```

## 说明

- 加速配置是**宿主机级别**的，主站 `main/` 和评测节点 `judgeserver/` 共用一份，两个目录都不用改。
- 只对 Docker Hub（`docker.io`）生效。本项目用到的 `mysql:8.0`、`redis:7-alpine`、
  `golang:1.26-alpine`、`node:20-alpine`、`nginx:alpine`、`docker:27-cli`、`gcc:13`、`alpine:latest`
  全部来自 Docker Hub，都在加速范围内。
- 修改 `daemon.json` 后**必须重启 docker**，否则不生效。
- 若 `/etc/docker/daemon.json` 原本不是合法 JSON（常见于手工编辑出错），脚本会中止并提示，
  不会覆盖原文件——按报错位置修好语法后重跑即可。

## 已内置的其它加速

仓库里的 Dockerfile 已经处理了构建阶段的软件源，和镜像加速互不冲突：

- Go 模块：`GOPROXY=https://mirrors.tencent.com/go/,https://goproxy.cn,direct`
- npm：`NPM_CONFIG_REGISTRY=https://mirrors.cloud.tencent.com/npm/`
- Alpine 软件源：`ARG APK_MIRROR=mirrors.aliyun.com`
- Debian 软件源：`ARG DEBIAN_MIRROR=mirrors.aliyun.com`

海外机器构建时用 `--build-arg APK_MIRROR=dl-cdn.alpinelinux.org` / `--build-arg DEBIAN_MIRROR=deb.debian.org`
切回官方源，两条路径共用同一份 Dockerfile。
