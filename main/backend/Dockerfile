# 依赖要求 Go 1.26 及以上，镜像 tag 需与同目录 go.mod 的 go 指令保持一致
#
# 镜像源：默认用阿里云（国内构建快）。海外机器构建时加
#   --build-arg APK_MIRROR=dl-cdn.alpinelinux.org
# 即可切回 Alpine 官方源，两条路径共用同一份 Dockerfile。
ARG APK_MIRROR=mirrors.aliyun.com

FROM golang:1.26-alpine AS build

# 本阶段不装任何 apk 包：go.mod 里没有 replace，依赖全部由 GOPROXY 以 HTTP 拉取，
# 不需要 git。若以后引入私有仓库或 replace 到 git 地址，在这里补：
#   ARG APK_MIRROR
#   RUN sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories && apk add --no-cache git

WORKDIR /src

# Go 模块代理（国内镜像，避免访问 proxy.golang.org 超时）
ENV GOPROXY=https://mirrors.tencent.com/go/,https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/akioi-cli ./cmd/akioi-cli

FROM alpine:latest

ARG APK_MIRROR

# Alpine 官方源（dl-cdn.alpinelinux.org）在国内很慢，单次 apk 安装要 60s+
RUN sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=build /out/ ./

RUN mkdir -p /app/logs /app/testdata /app/uploads

EXPOSE 8080

CMD ["/app/server"]
