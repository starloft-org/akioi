FROM gcc:13

# 判题需要 /usr/bin/time -v 采集内存峰值，基础镜像未包含
RUN apt-get update && apt-get install -y --no-install-recommends time && rm -rf /var/lib/apt/lists/*

# 用户代码编译与运行目录
WORKDIR /work
