FROM node:20-alpine AS build

WORKDIR /app

# npm 镜像（国内镜像，避免访问 registry.npmjs.org 超时）
ENV NPM_CONFIG_REGISTRY=https://mirrors.cloud.tencent.com/npm/

COPY package*.json ./
# 把 npm 下载缓存挂到 BuildKit 缓存：改 package.json 后重建复用已下载的 tarball，
# 不必再从零拉一遍依赖（首次构建仍有网络成本）
RUN --mount=type=cache,target=/root/.npm \
    npm install --prefer-offline --no-audit --no-fund

COPY . ./
RUN npm run build

FROM nginx:alpine

# certbot 负责自动申请与续期 Let's Encrypt 证书，
# openssl 用于还没有正式证书时生成临时自签名证书让 nginx 先跑起来
RUN apk add --no-cache certbot openssl

COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint-ssl.sh
RUN chmod +x /usr/local/bin/docker-entrypoint-ssl.sh

EXPOSE 80 443
ENTRYPOINT ["/usr/local/bin/docker-entrypoint-ssl.sh"]
