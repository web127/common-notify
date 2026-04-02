# Common-Notify 通知服务

企业内部通知服务，接收业务系统提交的外部 HTTP 通知请求，并可靠地投递到目标地址。

## 核心价值

- **屏蔽多供应商异构性**: 统一处理不同 API 地址、Header、Body 格式
- **屏蔽可靠性复杂性**: 自动重试、持久化、幂等性保证
- **屏蔽运维复杂性**: 统一监控、日志、配置管理

## 快速开始

### 构建

```bash
go build -o bin/common-notify main.go
```

### 运行

```bash
./bin/common-notify
```

服务将在 `:8080` 端口启动。

## API 文档

### 创建供应商配置

```bash
curl -X POST http://localhost:8080/vendors \
  -H "Content-Type: application/json" \
  -d '{
    "id": "ad-network-1",
    "name": "广告网络 1",
    "endpoint_url": "https://api.ad-network.com/webhook",
    "method": "POST",
    "headers": {
      "Authorization": "Bearer your-token",
      "X-API-Key": "your-api-key"
    },
    "timeout_ms": 5000,
    "skip_tls_verify": false
  }'
```

### 提交通知

```bash
curl -X POST http://localhost:8080/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "out_biz_no": "biz-20240402-12345",
    "vendor_id": "ad-network-1",
    "event_type": "registration",
    "payload": {
      "user_id": 123,
      "event": "registration"
    }
  }'
```

### 查询通知状态

```bash
curl http://localhost:8080/notifications/1
```

### 重试失败通知

```bash
curl -X POST http://localhost:8080/notifications/1/retry
```

## 架构

```
业务系统 -> API 层 -> 服务层 -> 数据层 (SQLite)
                          ^
                          |
                    Worker (定时任务)
                          |
                          v
                    外部供应商 API
```

## 技术栈

- **语言**: Go
- **Web 框架**: Gin
- **数据库**: SQLite
- **定时任务**: cron

## 可靠性保证

- **投递语义**: 至少一次 (At-least-once)
- **重试策略**: 指数退避 (1s, 2s, 4s, 8s, 16s, 32s)
- **幂等性**: 通过 `out_biz_no` + `vendor_id` 保证

## LICENSE

MIT
