# 使用官方的Go镜像作为构建环境
FROM golang:1.25.5-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制go.mod和go.sum文件
COPY go.mod .

# 下载依赖
RUN go mod tidy

# 复制源代码
COPY . .

# 构建应用
RUN go build -o notification-system ./cmd/main.go

# 使用轻量级的Alpine镜像作为运行环境
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 复制构建好的应用
COPY --from=builder /app/notification-system .

# 复制配置文件
COPY config.json .

# 暴露端口
EXPOSE 8080

# 设置环境变量
ENV NOTIFICATION_ENV=production

# 运行应用
CMD ["./notification-system"]
