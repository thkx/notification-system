# 通知系统

一个基于 Go 的通知服务，支持多渠道发送、批量投递、状态查询、WebSocket 实时推送，以及可选的 PostgreSQL 持久化。

## 当前能力

- 支持 `email`、`sms`、`inapp`、`social`
- 支持单条发送、批量发送、按条件查询、按 ID 查询
- 支持 WebSocket 实时广播已成功发送的通知
- 支持内存存储和 PostgreSQL 存储
- 支持通知去重、基础内容校验、重试与指标统计
- 支持 API Key 鉴权、请求体大小限制、HTTP 超时控制、WebSocket 来源白名单

## 项目结构

```text
notification-system/
├── cmd/                        # 入口程序
├── config/                     # 配置加载与校验
├── internal/
│   ├── api/                    # HTTP / WebSocket 服务
│   ├── channels/               # 渠道实现
│   ├── distribution/           # 通知处理与去重
│   ├── gateway/                # 网关与状态管理
│   ├── router/                 # 队列与实际投递
│   ├── services/               # 示例业务服务
│   └── storage/                # 内存 / PostgreSQL 存储
├── pkg/                        # 公共能力
├── config.json                 # 默认配置
├── config.production.json      # 生产配置示例
├── docker-compose.postgres.yml # PostgreSQL 本地依赖
├── Makefile
└── README.md
```

## 环境要求

- Go `1.25.5`

## 快速开始

### 1. 安装依赖

```bash
git clone https://github.com/thkx/notification-system.git
cd notification-system
```

### 2. 选择配置

默认读取规则：

- `NOTIFICATION_ENV` 未设置时，加载 `config.json`
- 如果设置了 `NOTIFICATION_ENV=production`，会优先读取 `config.production.json`
- 如果目标配置文件不存在，则回退到 `config.json`

示例：

```bash
export NOTIFICATION_ENV=development
```

### 3. 启动服务

```bash
go run ./cmd
```

也可以使用项目命令：

```bash
make run
```

如果希望启动时发送一组演示通知：

```bash
NOTIFICATION_RUN_DEMO=true go run ./cmd
```

## 常用命令

```bash
make fmt        # 格式化代码
make test       # 运行全部测试
make test-short # 运行短测试
make bench      # 运行 benchmark
make run        # 启动服务
```

## 配置说明

配置按“先默认值，再文件覆盖，最后环境变量覆盖”的顺序加载。

### `server`

```json
{
  "server": {
    "port": 8080,
    "readTimeoutMs": 5000,
    "writeTimeoutMs": 10000,
    "idleTimeoutMs": 60000,
    "maxBodyBytes": 1048576,
    "allowedOrigins": []
  }
}
```

字段说明：

- `port`: HTTP 服务端口
- `readTimeoutMs`: 请求读取超时
- `writeTimeoutMs`: 响应写入超时
- `idleTimeoutMs`: Keep-Alive 空闲超时
- `maxBodyBytes`: 单次请求体最大字节数
- `allowedOrigins`: 允许的额外 WebSocket 来源白名单

说明：

- WebSocket 默认允许无 `Origin` 的客户端，以及与请求 `Host` 同源的浏览器请求
- 如果前端和服务不在同源，需要把前端地址加入 `allowedOrigins`

### `security`

```json
{
  "security": {
    "requireApiKey": false,
    "apiKey": ""
  }
}
```

字段说明：

- `requireApiKey`: 是否强制 API Key 鉴权
- `apiKey`: 服务端校验的密钥

当 `requireApiKey=true` 时，请求需要带上以下任意一种头：

```http
X-API-Key: your-secret
```

或：

```http
Authorization: Bearer your-secret
```

生产环境建议：

- 使用 `config.production.json`，其中默认开启 `requireApiKey`
- 通过环境变量 `NOTIFICATION_API_KEY` 注入密钥，不要把真实密钥写进仓库

### `router`

```json
{
  "router": {
    "bufferSize": 1000,
    "workerCount": 3,
    "maxRetries": 3,
    "retryDelayMs": 100
  }
}
```

字段说明：

- `bufferSize`: 队列缓冲大小
- `workerCount`: 并发 worker 数量
- `maxRetries`: 队列满时的重试次数
- `retryDelayMs`: 队列重试的基础延迟

### `metrics`

```json
{
  "metrics": {
    "maxFailureRate": 0.2,
    "maxQueueUtilization": 0.8,
    "maxProcessingTime": 5000
  }
}
```

### `distribution`

```json
{
  "distribution": {
    "deduplicationTTL": 60
  }
}
```

同一个通知 ID 在去重窗口内会被视为重复请求。

### `store`

如果不配置 `store`，默认使用内存存储。

PostgreSQL 示例：

```json
{
  "store": {
    "type": "postgres",
    "dsn": "postgres://notification:notification@localhost:5432/notificationdb?sslmode=disable"
  }
}
```

说明：

- 当 `store.type=postgres` 时，服务会在启动时强制连接数据库
- 如果 PostgreSQL 初始化失败，服务会直接启动失败，不再静默回退到内存存储

### 环境变量

- `NOTIFICATION_ENV`: 运行环境名，对应 `config.<env>.json`
- `NOTIFICATION_RUN_DEMO`: 是否在启动时发送演示通知，支持 `true/false/1/0/yes/no`
- `NOTIFICATION_API_KEY`: 覆盖 `security.apiKey`
- `NOTIFICATION_ALLOWED_ORIGINS`: 覆盖 `server.allowedOrigins`，多个值用英文逗号分隔
- `EMAIL_PROVIDER`: `memory` 或 `sendgrid`
- `SENDGRID_API_KEY`: SendGrid Key
- `SENDGRID_FROM_EMAIL`: SendGrid 发件邮箱
- `SENDGRID_FROM_NAME`: SendGrid 发件人名称，可选
- `SMS_PROVIDER`: `memory` 或 `twilio`
- `TWILIO_ACCOUNT_SID`: Twilio Account SID
- `TWILIO_AUTH_TOKEN`: Twilio Auth Token
- `TWILIO_FROM_NUMBER`: Twilio 发信号码

Provider 说明：

- 当 `EMAIL_PROVIDER=sendgrid` 时，`Notification.UserID` 会被当成收件邮箱地址
- 当 `SMS_PROVIDER=twilio` 时，`Notification.UserID` 会被当成收件手机号
- 如果缺少 SDK 所需配置，发送会返回错误，不会再静默降级为成功

## 本地启动 PostgreSQL

```bash
docker compose -f docker-compose.postgres.yml up -d
```

## API

### 健康检查

```http
GET /health
```

响应：

```json
{
  "status": "ok",
  "service": "notification-system"
}
```

### 发送单条通知

```http
POST /api/notifications
Content-Type: application/json
```

示例：

```json
{
  "ID": "notif-1001",
  "UserID": "user-1",
  "Type": "order",
  "Content": "订单已完成",
  "Channels": ["email", "inapp"]
}
```

成功响应：

```json
{
  "status": "success"
}
```

说明：

- API 返回成功时，表示通知已经完成渠道处理，而不是仅仅入队
- 如果渠道发送失败，接口会返回错误，通知状态会被记为 `failed`

### 批量发送通知

```http
POST /api/notifications/batch
Content-Type: application/json
```

示例：

```json
[
  {
    "ID": "batch-1",
    "UserID": "user-1",
    "Type": "order",
    "Content": "订单通知",
    "Channels": ["email"]
  },
  {
    "ID": "batch-2",
    "UserID": "user-2",
    "Type": "payment",
    "Content": "支付通知",
    "Channels": ["sms"]
  }
]
```

成功响应：

```json
{
  "status": "success",
  "result": {
    "total": 2,
    "successful": 2,
    "failed": 0,
    "failedIds": []
  }
}
```

### 查询通知列表

```http
GET /api/notifications?userId=user-1&status=sent&sortBy=createdAt&order=desc&limit=20&offset=0
```

查询参数：

- `userId`: 可选
- `status`: 可选
- `sortBy`: 可选，支持 `createdAt`、`updatedAt`、`id`
- `order`: 可选，支持 `asc`、`desc`
- `limit`: 可选，非负整数；不传表示不限制
- `offset`: 可选，非负整数

### 查询单条通知

```http
GET /api/notifications/{id}
```

### WebSocket

```http
GET /ws
```

连接成功后会收到欢迎消息，后续在通知成功发送后会收到广播：

```json
{
  "type": "notification",
  "notification": {
    "ID": "notif-1001",
    "UserID": "user-1",
    "Type": "order",
    "Content": "您的订单 notif-1001 已处理完成，感谢您的购买！",
    "Channels": ["email", "inapp"],
    "Priority": 1,
    "Scheduled": "",
    "Status": "sent"
  }
}
```

## 示例请求

无鉴权开发模式：

```bash
curl -X POST http://localhost:8080/api/notifications \
  -H 'Content-Type: application/json' \
  -d '{
    "ID": "notif-1",
    "UserID": "user-1",
    "Type": "promotion",
    "Content": "周末活动开始了",
    "Channels": ["email", "sms"]
  }'
```

开启 API Key 时：

```bash
curl -X POST http://localhost:8080/api/notifications \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: your-secret' \
  -d '{
    "ID": "notif-2",
    "UserID": "user-2",
    "Type": "urgent",
    "Content": "请尽快处理工单",
    "Channels": ["inapp"]
  }'
```

## 测试

```bash
go test ./...
```

本项目当前测试覆盖了：

- 配置校验与环境变量覆盖
- HTTP API 基础行为
- 路由队列与优雅关闭
- 存储层查询与排序
- 网关的单条/批量发送流程

## 当前约束

- README 只描述当前已实现能力，不再宣称配置热更新
- `sendgrid` / `twilio` 已接入官方 Go SDK，但模型字段仍较轻量，当前使用 `UserID` 作为目标地址
- WebSocket 广播是服务内存级连接管理，未做多实例共享
- 当前没有分页查询能力，通知列表接口会返回全部匹配结果

## License

[LICENSE](./LICENSE)
