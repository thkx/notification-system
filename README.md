# 通知系统

一个基于Go语言的高性能通知系统，支持多种通知渠道，具有可扩展性和可靠性。

## 功能特性

- **多渠道支持**：支持邮件、短信、应用内通知和社交媒体通知
- **配置管理**：支持多环境配置和热更新
- **错误处理**：统一的错误处理机制，提供详细的错误信息
- **日志管理**：支持结构化日志和多种日志级别
- **队列处理**：使用带缓冲的通道提高性能
- **渠道插件化**：支持自定义渠道的注册和管理
- **安全检查**：通知内容的安全检查和验证
- **监控告警**：内置监控指标和告警机制

## 项目结构

```
notification-system/
├── cmd/                # 命令行入口
│   ├── main.go         # 主入口文件
│   └── main_test.go    # 测试文件
├── config/             # 配置管理
│   └── config.go       # 配置加载和管理
├── internal/           # 内部包
│   ├── analytics/      # 分析模块
│   ├── channels/       # 通知渠道
│   ├── distribution/   # 通知分发
│   ├── gateway/        # 通知网关
│   ├── router/         # 通知路由
│   └── services/       # 业务服务
├── pkg/                # 公共包
│   ├── di/             # 依赖注入
│   ├── errors/         # 错误处理
│   ├── logger/         # 日志管理
│   ├── metrics/        # 监控指标
│   ├── model/          # 数据模型
│   ├── retry/          # 重试机制
│   └── security/       # 安全检查
├── scripts/            # 脚本文件
│   ├── start.bat       # Windows启动脚本
│   └── start.sh        # Linux启动脚本
├── config.json         # 默认配置文件
├── go.mod              # Go模块文件
└── README.md           # 项目文档
```

## 快速开始

### 环境要求

- Go 1.18+

### 安装和运行

1. **克隆代码**

```bash
git clone https://github.com/thkx/notification-system.git
cd notification-system
```

2. **配置环境**

复制 `config.json` 文件并根据需要修改配置：

```bash
cp config.json config.development.json
```

3. **运行系统**

使用脚本启动系统：

```bash
# Windows
./scripts/start.bat

# Linux
./scripts/start.sh
```

或者直接运行：

```bash
go run cmd/main.go
```

### 配置说明

#### PostgreSQL 支持

如果使用 `storage.type` 设为 `postgres`，应在 `config.*.json` 加入：

```json
"storage": {
  "type": "postgres",
  "dsn": "postgres://notification:notification@localhost:5432/notificationdb?sslmode=disable"
}
```

可启动 PostgreSQL：

```bash
docker compose -f docker-compose.postgres.yml up -d
```

配置文件示例：

```json
{
  "server": {
    "port": 8080
  },
  "channels": {
    "email": {
      "enabled": true
    },
    "sms": {
      "enabled": true
    },
    "inapp": {
      "enabled": true
    },
    "social_media": {
      "enabled": true
    }
  },
  "environment": "development"
}
```

### 环境变量

- `NOTIFICATION_ENV`：运行环境，默认为 `development`
- `EMAIL_PROVIDER`：邮件供应商（`memory` / `sendgrid`），默认 `memory`
- `SENDGRID_API_KEY`：SendGrid API Key（可选）
- `SMS_PROVIDER`：短信供应商（`memory` / `twilio`），默认 `memory`
- `TWILIO_ACCOUNT_SID` / `TWILIO_AUTH_TOKEN`：Twilio 认证信息（可选）

## API 文档

### 通知网关

#### 查询通知列表

- `GET /api/notifications?userId={userId}&status={status}`
- `userId` 和 `status` 可选，支持按用户和状态过滤

#### 查看单条通知

- `GET /api/notifications/{id}`

#### 发送单个通知

```go
func (g *Gateway) SendNotification(notification *Notification) error
```

**参数**：
- `notification`：通知对象，包含ID、用户ID、类型、内容、渠道列表等信息

**返回值**：
- `error`：发送过程中的错误

#### 批量发送通知

```go
func (g *Gateway) SendBatchNotifications(notifications []*Notification) error
```

**参数**：
- `notifications`：通知对象列表

**返回值**：
- `error`：发送过程中的错误

### 渠道管理

#### 注册渠道

```go
func RegisterChannel(name string, factory ChannelFactory)
```

**参数**：
- `name`：渠道名称
- `factory`：渠道工厂函数

#### 获取渠道

```go
func GetChannel(name string) (Channel, error)
```

**参数**：
- `name`：渠道名称

**返回值**：
- `Channel`：渠道实例
- `error`：获取过程中的错误

### 安全检查

#### 验证通知内容

```go
func ValidateNotification(content string) *ValidationResult
```

**参数**：
- `content`：通知内容

**返回值**：
- `ValidationResult`：验证结果，包含是否有效和错误信息

#### 清理通知内容

```go
func SanitizeContent(content string) string
```

**参数**：
- `content`：通知内容

**返回值**：
- `string`：清理后的内容

## 扩展渠道

要添加自定义通知渠道，需要实现 `Channel` 接口：

```go
type Channel interface {
	// Send 发送通知
	Send(notification *model.Notification) error
	
	// Name 获取渠道名称
	Name() string
}
```

然后注册渠道：

```go
channels.RegisterChannel("custom", func() channels.Channel {
	return NewCustomChannel()
})
```

## 监控和告警

系统内置了监控指标，包括：

- 总通知数
- 成功通知数
- 失败通知数
- 渠道级别的指标
- 队列长度
- 处理时间

当失败率超过20%或队列长度超过100时，系统会触发告警。

## 测试

运行测试：

```bash
go test ./...
```

## 部署

### 容器化部署

项目包含Dockerfile，可以构建容器镜像：

```bash
docker build -t notification-system .
docker run -p 8080:8080 notification-system
```

### 集群部署

对于生产环境，建议使用Kubernetes进行集群部署，确保高可用性和可扩展性。

## 贡献

欢迎提交Issue和Pull Request！

## 许可证

[MIT License](LICENSE)
