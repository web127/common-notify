# common-notify

外部 HTTP 通知投递系统 - 接收业务系统提交的外部 HTTP 通知请求，并可靠地投递到目标地址。

## 需求背景

企业内部多个业务系统在关键事件发生时，需要调用外部系统供应商提供的 HTTP(S) API 进行通知。例如：
- 用户通过第三方广告系统引流并成功注册后，通知对应的广告系统
- 用户订阅付款成功后，通知 CRM 系统更改 Contact 状态
- 用户购买商品后，通知库存系统进行库存变更

不同供应商的 API：请求地址不同、Header/Body 格式不同。

业务系统本身：不需要关心外部 API 的返回值，只需确保通知请求能够被稳定、可靠地送达。

---

## 一、你对问题的理解

### 1.1 核心问题

**问题本质：** 业务系统与外部系统之间的可靠通知投递层。

**上游业务系统的痛点：**
- 每个业务系统都要自己实现重试、重试、重试逻辑
- 要处理网络超时、外部系统暂时不可用等各种异常情况
- 要自己保证通知不丢失，服务重启后要能恢复
- 多个业务系统重复造轮子

**common-notify 的价值：**
- 上游只负责「说什么」，common-notify 负责「怎么送到」。

### 1.2 核心诉求拆解

| 维度 | 业务诉求 | 技术体现 |
|-----|---------|---------|
| 可靠性 | 通知不能丢 | 先持久化再返回，服务重启可恢复 |
| 解耦 | 上游不用管投递细节 | 上游只发一个 HTTP 请求，后续交给 common-notify |
| 多态 | 支持不同外部 API 格式 | Header/Body 可自定义 |
| 幂等 | 上游重试安全 | 通过 out_biz_no 去重 |

---

## 二、整体架构与核心设计

### 2.1 系统架构

```
┌─────────────┐
│  业务系统    │
└──────┬──────┘
       │ HTTP POST
       ▼
┌─────────────────────────────────────────┐
│         common-notify 服务               │
│  ┌───────────────────────────────────┐  │
│  │  HTTP API 层                       │  │
│  │  - POST /api/v1/notifications      │  │
│  └───────────────┬───────────────────┘  │
│                  │ 写入                   │
│                  ▼                        │
│  ┌───────────────────────────────────┐  │
│  │  SQLite 数据库                      │  │
│  │  - notifications 表                 │  │
│  └───────────────┬───────────────────┘  │
│                  │ 读取                   │
│                  ▼                        │
│  ┌───────────────────────────────────┐  │
│  │  内存队列 + Worker 池               │  │
│  │  - 3 个并发 worker                  │  │
│  │  - 指数退避重试                      │  │
│  └───────────────┬───────────────────┘  │
│                  │ HTTP 请求              │
│                  ▼                        │
│         ┌─────────────────┐              │
│         │  外部供应商 API  │              │
│         └─────────────────┘              │
└─────────────────────────────────────────┘
```

### 2.2 核心组件

#### HTTP API 层

**接口：** `POST /api/v1/notifications`

**职责：**
- 接收上游业务系统的通知请求
- 幂等去重（通过 out_biz_no）
- 写入 SQLite 持久化
- 立即返回 202 Accepted

#### SQLite 存储层

**表结构：**
- `notifications` 表：存储所有通知任务
- 索引：status、event_type、next_retry_at

**职责：**
- 持久化所有通知任务
- 支持事务保证数据一致性
- 服务重启后可恢复 pending 任务

#### Worker 处理层

**Worker 池：** 3 个并发 worker

**处理流程：**
1. 从 DB 加载 pending 任务到内存队列
2. Worker 从队列取任务，标记为 processing
3. 发送 HTTP 请求到外部系统
4. 成功 → 标记为 success
5. 失败 → 指数退避重试（1s→2s→4s→8s→16s）
6. 超过 5 次 → 标记为 dead（死信）

### 2.3 数据模型

```go
type Notification struct {
    ID          int64
    OutBizNo    string          // 外部业务单号，幂等键
    EventType   string          // 事件类型
    Status      string          // pending/processing/success/dead
    TargetURL   string          // 目标 URL
    Method      string          // HTTP 方法
    Headers     string          // JSON 格式的 Header
    Body        string          // 请求 Body
    RetryCount  int             // 当前重试次数
    MaxRetries  int             // 最大重试次数
    LastError   string          // 最后一次错误信息
    NextRetryAt *time.Time      // 下次重试时间
}
```

---

## 三、关键工程决策与取舍说明

### 3.1 系统边界

**本系统解决的问题：**
- ✅ 业务系统与外部系统解耦
- ✅ 可靠投递（至少一次语义）
- ✅ 重试机制（指数退避）
- ✅ 死信队列（多次失败后放弃）
- ✅ 多上游/多事件类型支持
- ✅ 幂等性支持（通过 out_biz_no）

**本系统明确不解决的问题及原因：**

| 不解决的问题 | 原因 |
|-------------|------|
| 分布式部署 | MVP 阶段单点足够，简化设计 |
| 水平扩展 | MVP 阶段不需要，未来可以引入消息队列 |
| 消息顺序保证 | 业务场景通常不要求严格顺序，实现复杂度高 |
| 外部 API 的响应处理 | 需求明确说业务系统不需要关心返回值 |
| 监控、告警、Metrics | MVP 阶段不需要，后续可叠加 |
| 外部系统的幂等实现 | 那是外部系统的责任，本系统只提供 `out_biz_no` 支持 |

### 3.2 可靠性与失败处理

**投递语义：至少一次（At-least-once）**

设计选择理由：业务场景宁愿重复通知也不能丢失通知。外部系统需要根据 `out_biz_no` 实现幂等处理。

**外部系统失败处理策略：**
- **重试策略：** 指数退避，间隔依次为 1s → 2s → 4s → 8s → 16s
- **最大重试次数：** 5 次
- **失败后处理：** 超过最大重试次数后进入 `dead` 状态，不再自动重试，等待人工介入
- **持久化：** 所有状态变更都写入 SQLite，服务重启后可恢复

### 3.3 技术选型决策

#### 为什么选择 SQLite？

| 方案 | 优点 | 缺点 |
|-----|------|------|
| SQLite（已选） | 无依赖、易部署、事务支持 | 不支持高并发写入、单机 |
| Redis | 高性能、丰富的数据结构 | 持久化配置复杂、额外依赖 |
| RabbitMQ | 专业的消息队列、可靠性高 | 部署复杂、重量级 |
| 纯内存 + 本地文件 | 最简单 | 自己实现持久化逻辑易出错 |

**决策理由：** MVP 阶段读写量不大，SQLite 性能足够；无外部依赖，部署简单；事务支持保证数据一致性。

#### 为什么选择 Go？

- 项目目录在 go/src 下，用户有 Go 背景
- goroutine 适合 worker 场景
- 编译为单二进制，部署简单
- 标准库 HTTP client 功能完善

### 3.4 取舍：选择不采纳的"过度设计"

| 未采纳的设计 | 原因 |
|-------------|------|
| 引入 Redis/RabbitMQ | MVP 阶段不需要，SQLite 已足够，减少外部依赖 |
| 分布式架构设计 | 超出 MVP 范围，单点足够 |
| 完善的监控告警体系 | 需求未要求，MVP 阶段简化 |
| 消息顺序保证 | 实现复杂，业务不要求 |

### 3.5 未来演进方向（流量/复杂度增长时）

1. **引入消息队列（如 Redis List / RabbitMQ）** - 替代内存队列，支持分布式部署
2. **分库分表** - SQLite 替换为 PostgreSQL/MySQL，按 `event_type` 或 `out_biz_no` 分库
3. **增加调度器** - 独立的调度服务处理超时任务，与 worker 解耦
4. **监控告警** - 增加 Prometheus metrics + Grafana dashboard + 告警规则
5. **死信自动回放** - 支持人工触发或定时自动重试 dead 状态的任务
6. **多租户支持** - 按业务系统隔离，增加认证授权

---

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
