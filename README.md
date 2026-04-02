# common-notify

外部 HTTP 通知投递系统 - 接收业务系统提交的外部 HTTP 通知请求，并可靠地投递到目标地址。

## 需求背景

企业内部多个业务系统在关键事件发生时，需要调用外部系统供应商提供的 HTTP(S) API 进行通知。例如：
- 用户通过第三方广告系统引流并成功注册后，通知对应的广告系统
- 用户订阅付款成功后，通知 CRM 系统更改 Contact 状态
- 用户购买商品后，通知库存系统进行库存变更

不同供应商的 API：请求地址不同、Header/Body 格式不同。

业务系统本身：不需要关心外部 API 的返回值，只需确保通知请求能够被稳定、可靠地送达。

## 架构

- HTTP API 层：接收通知请求，幂等去重
- SQLite 存储：持久化通知任务
- Worker 池：并发投递，指数退避重试

## 核心设计要点

### 1. 系统边界

**本系统解决的问题：**
- 业务系统与外部系统解耦
- 可靠投递（至少一次语义）
- 重试机制（指数退避）
- 死信队列（多次失败后放弃）
- 多上游/多事件类型支持
- 幂等性支持（通过 out_biz_no）

**本系统明确不解决的问题及原因：**

| 不解决的问题 | 原因 |
|-------------|------|
| 分布式部署 | MVP 阶段单点足够，简化设计 |
| 水平扩展 | MVP 阶段不需要，未来可以引入消息队列 |
| 消息顺序保证 | 业务场景通常不要求严格顺序，实现复杂度高 |
| 外部 API 的响应处理 | 需求明确说业务系统不需要关心返回值 |
| 监控、告警、Metrics | MVP 阶段不需要，后续可叠加 |
| 外部系统的幂等实现 | 那是外部系统的责任，本系统只提供 `out_biz_no` 支持 |

### 2. 可靠性与失败处理

**投递语义：至少一次（At-least-once）**

设计选择理由：业务场景宁愿重复通知也不能丢失通知。外部系统需要根据 `out_biz_no` 实现幂等处理。

**外部系统失败处理策略：**
- **重试策略：** 指数退避，间隔依次为 1s → 2s → 4s → 8s → 16s
- **最大重试次数：** 5 次
- **失败后处理：** 超过最大重试次数后进入 `dead` 状态，不再自动重试，等待人工介入
- **持久化：** 所有状态变更都写入 SQLite，服务重启后可恢复

### 3. 取舍与演进

**选择不采纳的"过度设计"：**

| 未采纳的设计 | 原因 |
|-------------|------|
| 引入 Redis/RabbitMQ | MVP 阶段不需要，SQLite 已足够，减少外部依赖 |
| 分布式架构设计 | 超出 MVP 范围，单点足够 |
| 完善的监控告警体系 | 需求未要求，MVP 阶段简化 |
| 消息顺序保证 | 实现复杂，业务不要求 |

**未来演进方向（流量/复杂度增长时）：**
1. **引入消息队列（如 Redis List / RabbitMQ）** - 替代内存队列，支持分布式部署
2. **分库分表** - SQLite 替换为 PostgreSQL/MySQL，按 `event_type` 或 `out_biz_no` 分库
3. **增加调度器** - 独立的调度服务处理超时任务，与 worker 解耦
4. **监控告警** - 增加 Prometheus metrics + Grafana dashboard + 告警规则
5. **死信自动回放** - 支持人工触发或定时自动重试 dead 状态的任务
6. **多租户支持** - 按业务系统隔离，增加认证授权

### 技术选型说明

**为什么选择 SQLite？**

| 方案 | 优点 | 缺点 |
|-----|------|------|
| SQLite（已选） | 无依赖、易部署、事务支持 | 不支持高并发写入、单机 |
| Redis | 高性能、丰富的数据结构 | 持久化配置复杂、额外依赖 |
| RabbitMQ | 专业的消息队列、可靠性高 | 部署复杂、重量级 |
| 纯内存 + 本地文件 | 最简单 | 自己实现持久化逻辑易出错 |

**为什么选择 Go？**
- 项目目录在 go/src 下，用户有 Go 背景
- goroutine 适合 worker 场景
- 编译为单二进制，部署简单
- 标准库 HTTP client 功能完善

## 快速开始

```bash
# 构建
go build -o common-notify .

# 运行
./common-notify

# 发送测试请求
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "out_biz_no": "test-001",
    "event_type": "user.registered",
    "target_url": "https://httpbin.org/post",
    "method": "POST",
    "headers": {"Content-Type": "application/json"},
    "body": "{\"user_id\": 123}"
  }'
```

## API 文档

### POST /api/v1/notifications

创建通知请求。

**请求体：**
```json
{
  "out_biz_no": "uuid-xxx-xxx",      // 可选但推荐，外部业务单号，用于幂等
  "event_type": "user.registered",     // 事件类型，标识上游业务和事件
  "target_url": "https://external-api.com/callback",
  "method": "POST",
  "headers": {
    "Content-Type": "application/json",
    "X-API-Key": "secret"
  },
  "body": "{\"user_id\": 123}"
}
```

**响应（202 Accepted）：**
```json
{
  "id": 1,
  "status": "pending",
  "message": "notification accepted"
}
```

**幂等命中响应（200 OK）：**
```json
{
  "id": 1,
  "status": "success",
  "message": "notification already exists (idempotent)"
}
```

## 运行测试

```bash
# 运行所有测试
go test -v ./internal/...

# 运行特定包测试
go test -v ./internal/store
```

## 目录结构

```
common-notify/
├── go.mod
├── go.sum
├── main.go
├── AI_USAGE.md
├── internal/
│   ├── model/
│   │   ├── notification.go       # 数据模型
│   │   └── notification_test.go  # 模型测试
│   ├── store/
│   │   ├── sqlite.go             # SQLite 存储层
│   │   └── sqlite_test.go        # 存储层测试
│   ├── worker/
│   │   ├── worker.go             # Worker 处理逻辑
│   │   └── worker_test.go        # Worker 测试
│   └── api/
│       ├── handler.go            # HTTP API 处理
│       └── handler_test.go       # API 测试
└── docs/
    └── superpowers/
        ├── specs/
        │   └── 2026-04-02-common-notify-design.md  # 详细设计文档
        └── plans/
            └── 2026-04-02-common-notify-implementation.md  # 实施计划
```

## 设计文档

详见 [docs/superpowers/specs/2026-04-02-common-notify-design.md](docs/superpowers/specs/2026-04-02-common-notify-design.md)
