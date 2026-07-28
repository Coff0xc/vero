# 多阶段构建 - 最小化镜像大小
FROM golang:1.26-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 构建二进制
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o redcell \
    ./cmd/redcell

# 最终镜像 - 基于 Alpine
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    curl \
    nmap \
    nmap-scripts \
    && rm -rf /var/cache/apk/*

# 创建非 root 用户
RUN addgroup -g 1000 redcell && \
    adduser -D -u 1000 -G redcell redcell

# 工作目录
WORKDIR /app

# 从构建阶段复制二进制
COPY --from=builder /build/redcell /usr/local/bin/redcell
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# 复制配置文件
COPY --chown=redcell:redcell wordlists/ /app/wordlists/

# 数据卷
VOLUME ["/app/data"]

# 切换到非 root 用户
USER redcell

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8000/ || exit 1

# 默认端口
EXPOSE 8000

# 环境变量
ENV REDCELL_DB=/app/data/redcell.db \
    REDCELL_PORT=8000

# 启动命令
ENTRYPOINT ["redcell"]
CMD ["-port", "8000", "-db", "/app/data/redcell.db"]
