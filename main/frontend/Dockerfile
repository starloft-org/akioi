FROM node:20-alpine AS build

WORKDIR /app

# npm 镜像（国内镜像，避免访问 registry.npmjs.org 超时）
ENV NPM_CONFIG_REGISTRY=https://mirrors.cloud.tencent.com/npm/

COPY package*.json ./
RUN npm install --prefer-offline --no-audit --no-fund

COPY . ./
RUN npm run build

FROM nginx:alpine

COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
