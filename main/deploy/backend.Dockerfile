# 依赖要求 Go 1.26 及以上，镜像 tag 需与 backend/go.mod 的 go 指令保持一致
FROM golang:1.26-alpine AS build

RUN apk add --no-cache git

WORKDIR /src

# Go 模块代理（国内镜像，避免访问 proxy.golang.org 超时）
ENV GOPROXY=https://mirrors.tencent.com/go/,https://goproxy.cn,direct

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/akioi-cli ./cmd/akioi-cli

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=build /out/ ./

RUN mkdir -p /app/logs /app/testdata /app/uploads

EXPOSE 8080

CMD ["/app/server"]
