# Common-Notify 外部 HTTP 通知投递系统 - 设计文档

## 1. 问题理解与设计理念

### 1.1 核心要解决的问题

1. **业务系统与外部系统解耦**


   **术语说明：** 本文档中的「业务系统」指 common-notify 的上游系统（调用 common-notify API 的一方）。

   **上游业务系统只需要做：**
   - 构造一个 HTTP 请求调用 `POST /api/v1/notifications`
   - 把目标 URL、Header、Body 告诉 common-notify
   - 收到 202 响应就可以返回了，不用管后续

   **common-notify 帮助上游屏蔽的复杂性：**

   | 复杂细节 | common-notify 的处理 |
   |---------|---------------------|
   | 重试逻辑 | 指数退避重试（1s→2s→4s→8s→16s），最多5次 |
   | 错误处理 | 网络超时、5xx 错误自动重试，4xx 错误记录后进入死信 |
   | 持久化 | 收到请求先写 SQLite 再返回，服务重启不丢失 |
   | 并发控制 | 多 worker 并发投递，控制并发量 |
   | 状态管理 | 跟踪每个通知的状态（pending/processing/success/dead） |
   | 幂等支持 | 通过 out_biz_no 去重，上游重试不用担心重复投递 |

   **设计理念：** 上游只负责「说什么」，common-notify 负责「怎么送到」。

2. **可靠投递保证**
   - 确保通知请求能稳定送达外部系统，即使外部系统暂时不可用
   - 接收请求后先持久化再返回，确保不丢失

3. **多上游支持**
   - 支持多个上游业务系统接入
   - 支持不同的事件类型标识
   - 支持不同的外部 API 格式（Header/Body 自定义）

### 1.2 设计理念

- **简单性优先（YAGNI）** - MVP 阶段只做必要的功能，不做过度设计
- **持久化保证** - 接收请求后先落盘再返回，确保不丢失
- **至少一次投递（At-least-once）** - 宁愿重复投递也不丢失（外部系统需实现幂等）
- **幂等性支持** - 通过 `out_biz_no` 帮助上游实现重试安全

---

## 2. 整体架构与核心设计

### 2.1 系统架构图

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

**响应：**
- 成功：202 Accepted，返回通知 ID
- 幂等命中：200 OK，返回已有通知记录

#### 数据模型（SQLite）

```sql
CREATE TABLE notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    out_biz_no TEXT UNIQUE,               -- 外部业务单号，幂等键，唯一索引
    event_type TEXT NOT NULL,             -- 事件类型
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL,                  -- pending, processing, success, dead
    target_url TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'POST',
    headers TEXT,                           -- JSON 格式
    body TEXT,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 5,
    last_error TEXT,
    next_retry_at TIMESTAMP
);

-- 索引
CREATE INDEX idx_notifications_status ON notifications(status);
CREATE INDEX idx_notifications_event_type ON notifications(event_type);
CREATE INDEX idx_notifications_next_retry_at ON notifications(next_retry_at);
```

#### Worker 处理逻辑

1. 服务启动时从 DB 加载所有 `pending` 状态的任务到内存队列
2. 3 个并发 worker 循环从队列取任务
3. 任务标记为 `processing`
4. 发送 HTTP 请求到外部系统
5. 成功 → 标记为 `success`
6. 失败：
   - 重试次数 +1
   - 计算下次重试时间（指数退避：1s → 2s → 4s → 8s → 16s）
   - 未超最大重试 → 放回 `pending`，更新 `next_retry_at`
   - 超最大重试 → 标记为 `dead`（死信，等待人工介入）

#### 幂等性处理逻辑

1. 收到请求时，先检查 `out_biz_no`
2. 如果 `out_biz_no` 已存在且状态不是 `dead` → 直接返回已有记录，不重复处理
3. 如果不存在 → 创建新记录

---

## 3. 系统边界

### 3.1 本系统解决的问题

- 业务系统与外部系统解耦
- 可靠投递（至少一次语义）
- 重试机制（指数退避）
- 死信队列（多次失败后放弃）
- 多上游/多事件类型支持
- 幂等性支持（通过 out_biz_no）

### 3.2 本系统明确不解决的问题及原因

| 不解决的问题 | 原因 |
|-------------|------|
| 分布式部署 | MVP 阶段单点足够，简化设计 |
| 水平扩展 | MVP 阶段不需要，未来可以引入消息队列 |
| 消息顺序保证 | 业务场景通常不要求严格顺序，实现复杂度高 |
| 外部 API 的响应处理 | 需求明确说业务系统不需要关心返回值 |
| 监控、告警、Metrics | MVP 阶段不需要，后续可叠加 |
| 外部系统的幂等实现 | 那是外部系统的责任，本系统只提供 `out_biz_no` 支持 |

---

## 4. 可靠性与失败处理

### 4.1 投递语义

**至少一次（At-least-once）**

设计选择理由：业务场景宁愿重复通知也不能丢失通知。外部系统需要根据 `out_biz_no` 实现幂等处理。

### 4.2 外部系统失败处理策略

- **重试策略：** 指数退避，间隔依次为 1s → 2s → 4s → 8s → 16s
- **最大重试次数：** 5 次
- **失败后处理：** 超过最大重试次数后进入 `dead` 状态，不再自动重试，等待人工介入
- **持久化：** 所有状态变更都写入 SQLite，服务重启后可恢复

---

## 5. 取舍与演进

### 5.1 选择不采纳的"过度设计"

| 未采纳的设计 | 原因 |
|-------------|------|
| 引入 Redis/RabbitMQ | SQLite 已足够，减少外部依赖 |
| 分布式架构 | MVP 不需要，单点足够 |
| 完善的监控告警体系 | MVP 阶段不需要 |
| 消息顺序保证 | 实现复杂，业务不要求 |
| 自动 dead letter 回放 | 人工介入更可控，MVP 简化 |

### 5.2 未来演进方向（流量/复杂度增长时）

1. **引入消息队列（如 Redis List / RabbitMQ）** - 替代内存队列，支持分布式部署
2. **分库分表** - SQLite 替换为 PostgreSQL/MySQL，按 `event_type` 或 `out_biz_no` 分库
3. **增加调度器** - 独立的调度服务处理超时任务，与 worker 解耦
4. **监控告警** - 增加 Prometheus metrics + Grafana  dashboard + 告警规则
5. **死信自动回放** - 支持人工触发或定时自动重试 dead 状态的任务
6. **多租户支持** - 按业务系统隔离，增加认证授权

---

## 6. 技术选型说明

### 6.1 为什么选择 SQLite？

**选择理由：**
- 无额外依赖，单文件存储，部署简单
- Go 生态支持完善（mattn/go-sqlite3）
- MVP 阶段读写量不大，SQLite 性能足够
- 事务支持，保证数据一致性

**替代方案对比：**
| 方案 | 优点 | 缺点 |
|-----|------|------|
| SQLite（已选） | 无依赖、易部署、事务支持 | 不支持高并发写入、单机 |
| Redis | 高性能、丰富的数据结构 | 持久化配置复杂、额外依赖 |
| RabbitMQ | 专业的消息队列、可靠性高 | 部署复杂、重量级 |
| 纯内存 + 本地文件 | 最简单 | 自己实现持久化逻辑易出错 |

### 6.2 为什么选择 Go？

- 从项目目录 `go/src` 可以看出用户有 Go 背景
- 并发模型（goroutine）适合 worker 场景
- 编译为单二进制，部署简单
- 标准库 HTTP client 功能完善
