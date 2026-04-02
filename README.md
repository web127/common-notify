# Common-Notify

一个企业级 HTTP 通知投递服务，用于可靠地将业务系统的通知投递到外部供应商 API。

## 特性

- **多供应商配置管理**：动态配置不同供应商的 API 格式（URL、Header、Body）
- **可靠投递**：至少一次（at-least-once）投递语义
- **固定间隔重试**：可配置的重试策略
- **持久化存储**：基于 SQLite，所有通知都会持久化
- **HTTP API**：简洁的 RESTful API 供业务系统调用

## 快速开始

### 编译

```bash
go build -o bin/common-notify .
```

### 运行

```bash
./bin/common-notify -db common_notify.db -port :8080 -worker-interval 5 -batch-size 10
```

参数说明：
- `-db`: SQLite 数据库文件路径（默认：`common_notify.db`）
- `-port`: HTTP 服务端口（默认：`:8080`）
- `-worker-interval`: Worker 扫描间隔，单位秒（默认：`5`）
- `-batch-size`: 每次处理的通知数量（默认：`10`）

## 使用示例

### 1. 创建供应商配置

```bash
curl -X POST http://localhost:8080/api/v1/providers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ad_system",
    "base_url": "https://ad-provider.example.com/api/conversion",
    "method": "POST",
    "headers": {
      "Authorization": "Bearer your-token",
      "Content-Type": "application/json"
    }
  }'
```

### 2. 发送通知

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "provider_id": 1,
    "body": "{\"user_id\": \"12345\", \"event\": \"register\"}",
    "max_retry": 5,
    "retry_interval": 60
  }'
```

### 3. 查询通知状态

```bash
# 查询成功的通知
curl http://localhost:8080/api/v1/notifications/status/success

# 查询失败的通知
curl http://localhost:8080/api/v1/notifications/status/failed

# 查询单个通知
curl http://localhost:8080/api/v1/notifications/1
```

### 4. 手动重试失败通知

```bash
curl -X POST http://localhost:8080/api/v1/notifications/1/retry
```

## 配置说明

### 通知参数

创建通知时可以指定以下参数：

| 参数 | 类型 | 必填 | 说明 |
|-----|------|-----|-----|
| `provider_id` | int64 | 是 | 供应商 ID |
| `url` | string | 否 | 完整请求 URL（覆盖供应商 base_url） |
| `method` | string | 否 | HTTP 方法（覆盖供应商配置） |
| `headers` | map | 否 | 请求头（合并供应商配置） |
| `body` | string | 否 | 请求体 |
| `max_retry` | int | 否 | 最大重试次数（默认 5） |
| `retry_interval` | int | 否 | 重试间隔秒数（默认 60） |

### 通知状态

| 状态 | 说明 |
|-----|------|
| `pending` | 待发送 |
| `sending` | 发送中 |
| `success` | 发送成功 |
| `failed` | 发送失败（已达最大重试次数） |

## 项目结构

```
.
├── main.go                 # 程序入口
├── DESIGN.md               # 设计文档
├── README.md               # 本文件
├── go.mod
├── go.sum
└── internal/
    ├── model/              # 数据模型
    │   └── model.go
    ├── repository/         # 数据访问层
    │   ├── db.go
    │   ├── provider_repo.go
    │   └── notification_repo.go
    ├── service/            # 业务逻辑层
    │   ├── provider_service.go
    │   └── notification_service.go
    ├── api/                # HTTP API 层
    │   └── handler.go
    └── worker/             # 后台 Worker
        └── worker.go
```

## 设计决策

详细的设计说明请参考 [DESIGN.md](./DESIGN.md)。

核心决策：
- **持久化**：使用 SQLite 作为轻量级数据存储
- **投递语义**：至少一次（at-least-once）
- **重试策略**：固定间隔重试
- **架构风格**：简洁的分层架构（API → Service → Repository）

## 许可证

MIT License
