# Common-Notify 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现一个外部 HTTP 通知投递系统的 MVP，使用 Go + SQLite，支持幂等、重试、死信队列。

**Architecture:** 单体 Go 服务，HTTP API 层接收请求写入 SQLite，内存队列 + 多 worker 并发投递，指数退避重试。

**Tech Stack:** Go 1.21+, SQLite (mattn/go-sqlite3), 标准库 net/http

---

## 项目文件结构

```
common-notify/
├── go.mod
├── go.sum
├── main.go
├── internal/
│   ├── model/
│   │   └── notification.go    # 数据模型
│   ├── store/
│   │   └── sqlite.go          # SQLite 存储层
│   ├── worker/
│   │   └── worker.go          # Worker 处理逻辑
│   └── api/
│       └── handler.go         # HTTP API 处理
├── docs/
│   └── superpowers/
│       ├── specs/
│       │   └── 2026-04-02-common-notify-design.md
│       └── plans/
│           └── 2026-04-02-common-notify-implementation.md
├── design_requirement.md
├── origin_job_desc.md
└── README.md
```

---

### Task 1: 初始化 Go 项目

**Files:**
- Create: `go.mod`

- [ ] **Step 1: 初始化 go.mod**

```bash
cd /Users/bytedance/go/src/code.byted.org/fintech_cf/common-notify
go mod init code.byted.org/fintech_cf/common-notify
```

- [ ] **Step 2: 添加 SQLite 依赖**

```bash
go get github.com/mattn/go-sqlite3
```

- [ ] **Step 3: 验证 go.mod 创建成功**

检查 `go.mod` 内容应类似：
```
module code.byted.org/fintech_cf/common-notify

go 1.21

require github.com/mattn/go-sqlite3 v1.14.18
```

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "feat: initialize go module with sqlite dependency"
```

---

### Task 2: 定义数据模型

**Files:**
- Create: `internal/model/notification.go`

- [ ] **Step 1: 创建目录结构**

```bash
mkdir -p /Users/bytedance/go/src/code.byted.org/fintech_cf/common-notify/internal/model
```

- [ ] **Step 2: 编写数据模型代码**

```go
package model

import "time"

// NotificationStatus 通知状态
type NotificationStatus string

const (
	StatusPending    NotificationStatus = "pending"
	StatusProcessing NotificationStatus = "processing"
	StatusSuccess    NotificationStatus = "success"
	StatusDead       NotificationStatus = "dead"
)

// Notification 通知任务
type Notification struct {
	ID           int64              `json:"id"`
	OutBizNo     string             `json:"out_biz_no"`
	EventType    string             `json:"event_type"`
	CreatedAt    time.Time          `json:"created_at"`
	Status       NotificationStatus `json:"status"`
	TargetURL    string             `json:"target_url"`
	Method       string             `json:"method"`
	Headers      string             `json:"headers"` // JSON string
	Body         string             `json:"body"`
	RetryCount   int                `json:"retry_count"`
	MaxRetries   int                `json:"max_retries"`
	LastError    string             `json:"last_error"`
	NextRetryAt  *time.Time         `json:"next_retry_at"`
}

// CreateNotificationRequest 创建通知请求
type CreateNotificationRequest struct {
	OutBizNo  string            `json:"out_biz_no"`
	EventType string            `json:"event_type"`
	TargetURL string            `json:"target_url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
}

// CreateNotificationResponse 创建通知响应
type CreateNotificationResponse struct {
	ID      int64              `json:"id"`
	Status  NotificationStatus `json:"status"`
	Message string             `json:"message"`
}
```

- [ ] **Step 3: 验证文件创建成功**

- [ ] **Step 4: Commit**

```bash
git add internal/model/notification.go
git commit -m "feat: add notification data model"
```

---

### Task 3: 实现 SQLite 存储层

**Files:**
- Create: `internal/store/sqlite.go`

- [ ] **Step 1: 创建目录结构**

```bash
mkdir -p /Users/bytedance/go/src/code.byted.org/fintech_cf/common-notify/internal/store
```

- [ ] **Step 2: 编写 SQLite 存储代码**

```go
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"code.byted.org/fintech_cf/common-notify/internal/model"
)

const schema = `
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    out_biz_no TEXT UNIQUE,
    event_type TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL,
    target_url TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'POST',
    headers TEXT,
    body TEXT,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 5,
    last_error TEXT,
    next_retry_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);
CREATE INDEX IF NOT EXISTS idx_notifications_event_type ON notifications(event_type);
CREATE INDEX IF NOT EXISTS idx_notifications_next_retry_at ON notifications(next_retry_at);
`

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) CreateNotification(req *model.CreateNotificationRequest) (*model.Notification, error) {
	headersJSON, err := json.Marshal(req.Headers)
	if err != nil {
		return nil, err
	}

	if req.Method == "" {
		req.Method = "POST"
	}

	result, err := s.db.Exec(`
		INSERT INTO notifications (out_biz_no, event_type, status, target_url, method, headers, body, max_retries)
		VALUES (?, ?, ?, ?, ?, ?, ?, 5)
	`, req.OutBizNo, req.EventType, model.StatusPending, req.TargetURL, req.Method, string(headersJSON), req.Body)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return s.GetNotificationByID(id)
}

func (s *SQLiteStore) GetNotificationByOutBizNo(outBizNo string) (*model.Notification, error) {
	row := s.db.QueryRow(`
		SELECT id, out_biz_no, event_type, created_at, status, target_url, method, headers, body, retry_count, max_retries, last_error, next_retry_at
		FROM notifications WHERE out_biz_no = ?
	`, outBizNo)
	return scanNotification(row)
}

func (s *SQLiteStore) GetNotificationByID(id int64) (*model.Notification, error) {
	row := s.db.QueryRow(`
		SELECT id, out_biz_no, event_type, created_at, status, target_url, method, headers, body, retry_count, max_retries, last_error, next_retry_at
		FROM notifications WHERE id = ?
	`, id)
	return scanNotification(row)
}

func (s *SQLiteStore) ListPendingNotifications(limit int) ([]*model.Notification, error) {
	rows, err := s.db.Query(`
		SELECT id, out_biz_no, event_type, created_at, status, target_url, method, headers, body, retry_count, max_retries, last_error, next_retry_at
		FROM notifications
		WHERE status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)
		ORDER BY created_at ASC
		LIMIT ?
	`, model.StatusPending, time.Now(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []*model.Notification
	for rows.Next() {
		n, err := scanNotifications(rows)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	return notifications, nil
}

func (s *SQLiteStore) UpdateStatus(id int64, status model.NotificationStatus) error {
	_, err := s.db.Exec(`UPDATE notifications SET status = ? WHERE id = ?`, status, id)
	return err
}

func (s *SQLiteStore) MarkSuccess(id int64) error {
	_, err := s.db.Exec(`UPDATE notifications SET status = ? WHERE id = ?`, model.StatusSuccess, id)
	return err
}

func (s *SQLiteStore) MarkDead(id int64, lastError string) error {
	_, err := s.db.Exec(`UPDATE notifications SET status = ?, last_error = ? WHERE id = ?`, model.StatusDead, lastError, id)
	return err
}

func (s *SQLiteStore) ScheduleRetry(id int64, retryCount int, nextRetryAt time.Time, lastError string) error {
	_, err := s.db.Exec(`
		UPDATE notifications
		SET status = ?, retry_count = ?, next_retry_at = ?, last_error = ?
		WHERE id = ?
	`, model.StatusPending, retryCount, nextRetryAt, lastError, id)
	return err
}

func scanNotification(row *sql.Row) (*model.Notification, error) {
	var n model.Notification
	var nextRetryAt sql.NullTime
	err := row.Scan(
		&n.ID, &n.OutBizNo, &n.EventType, &n.CreatedAt, &n.Status,
		&n.TargetURL, &n.Method, &n.Headers, &n.Body, &n.RetryCount,
		&n.MaxRetries, &n.LastError, &nextRetryAt,
	)
	if err != nil {
		return nil, err
	}
	if nextRetryAt.Valid {
		n.NextRetryAt = &nextRetryAt.Time
	}
	return &n, nil
}

func scanNotifications(rows *sql.Rows) (*model.Notification, error) {
	var n model.Notification
	var nextRetryAt sql.NullTime
	err := rows.Scan(
		&n.ID, &n.OutBizNo, &n.EventType, &n.CreatedAt, &n.Status,
		&n.TargetURL, &n.Method, &n.Headers, &n.Body, &n.RetryCount,
		&n.MaxRetries, &n.LastError, &nextRetryAt,
	)
	if err != nil {
		return nil, err
	}
	if nextRetryAt.Valid {
		n.NextRetryAt = &nextRetryAt.Time
	}
	return &n, nil
}
```

- [ ] **Step 3: 验证代码编译通过**

```bash
go build ./internal/store
```

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlite.go
git commit -m "feat: implement sqlite store layer"
```

---

### Task 4: 实现 Worker 处理逻辑

**Files:**
- Create: `internal/worker/worker.go`

- [ ] **Step 1: 创建目录结构**

```bash
mkdir -p /Users/bytedance/go/src/code.byted.org/fintech_cf/common-notify/internal/worker
```

- [ ] **Step 2: 编写 Worker 代码**

```go
package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/store"
)

type Worker struct {
	store     *store.SQLiteStore
	queue     chan *model.Notification
	workerNum int
	client    *http.Client
}

func NewWorker(store *store.SQLiteStore, workerNum int, queueSize int) *Worker {
	return &Worker{
		store:     store,
		queue:     make(chan *model.Notification, queueSize),
		workerNum: workerNum,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (w *Worker) Start() {
	for i := 0; i < w.workerNum; i++ {
		go w.work()
	}
	go w.loadPendingTasks()
	go w.pollPendingTasks()
}

func (w *Worker) Submit(n *model.Notification) {
	select {
	case w.queue <- n:
	default:
		log.Printf("queue full, dropped notification: %d", n.ID)
	}
}

func (w *Worker) loadPendingTasks() {
	notifications, err := w.store.ListPendingNotifications(100)
	if err != nil {
		log.Printf("failed to load pending notifications: %v", err)
		return
	}
	for _, n := range notifications {
		w.Submit(n)
	}
}

func (w *Worker) pollPendingTasks() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		notifications, err := w.store.ListPendingNotifications(100)
		if err != nil {
			log.Printf("failed to poll pending notifications: %v", err)
			continue
		}
		for _, n := range notifications {
			w.Submit(n)
		}
	}
}

func (w *Worker) work() {
	for n := range w.queue {
		w.processNotification(n)
	}
}

func (w *Worker) processNotification(n *model.Notification) {
	if err := w.store.UpdateStatus(n.ID, model.StatusProcessing); err != nil {
		log.Printf("failed to update status to processing: %v", err)
		return
	}

	err := w.sendHTTPRequest(n)
	if err == nil {
		if err := w.store.MarkSuccess(n.ID); err != nil {
			log.Printf("failed to mark success: %v", err)
		}
		return
	}

	retryCount := n.RetryCount + 1
	if retryCount >= n.MaxRetries {
		if err := w.store.MarkDead(n.ID, err.Error()); err != nil {
			log.Printf("failed to mark dead: %v", err)
		}
		return
	}

	nextRetryAt := time.Now().Add(calculateBackoff(retryCount))
	if err := w.store.ScheduleRetry(n.ID, retryCount, nextRetryAt, err.Error()); err != nil {
		log.Printf("failed to schedule retry: %v", err)
	}
}

func (w *Worker) sendHTTPRequest(n *model.Notification) error {
	var headers map[string]string
	if n.Headers != "" {
		json.Unmarshal([]byte(n.Headers), &headers)
	}

	req, err := http.NewRequest(n.Method, n.TargetURL, bytes.NewReader([]byte(n.Body)))
	if err != nil {
		return err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("HTTP status %d", resp.StatusCode)
}

func calculateBackoff(retryCount int) time.Duration {
	return time.Duration(1<<retryCount) * time.Second
}
```

- [ ] **Step 3: 验证代码编译通过**

```bash
go build ./internal/worker
```

- [ ] **Step 4: Commit**

```bash
git add internal/worker/worker.go
git commit -m "feat: implement worker with exponential backoff"
```

---

### Task 5: 实现 HTTP API 处理

**Files:**
- Create: `internal/api/handler.go`

- [ ] **Step 1: 创建目录结构**

```bash
mkdir -p /Users/bytedance/go/src/code.byted.org/fintech_cf/common-notify/internal/api
```

- [ ] **Step 2: 编写 HTTP Handler 代码**

```go
package api

import (
	"encoding/json"
	"net/http"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/store"
	"code.byted.org/fintech_cf/common-notify/internal/worker"
)

type Handler struct {
	store  *store.SQLiteStore
	worker *worker.Worker
}

func NewHandler(store *store.SQLiteStore, worker *worker.Worker) *Handler {
	return &Handler{store: store, worker: worker}
}

func (h *Handler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req model.CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.EventType == "" || req.TargetURL == "" {
		http.Error(w, "event_type and target_url are required", http.StatusBadRequest)
		return
	}

	if req.OutBizNo != "" {
		if existing, _ := h.store.GetNotificationByOutBizNo(req.OutBizNo); existing != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(model.CreateNotificationResponse{
				ID:      existing.ID,
				Status:  existing.Status,
				Message: "notification already exists (idempotent)",
			})
			return
		}
	}

	n, err := h.store.CreateNotification(&req)
	if err != nil {
		http.Error(w, "failed to create notification", http.StatusInternalServerError)
		return
	}

	h.worker.Submit(n)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(model.CreateNotificationResponse{
		ID:      n.ID,
		Status:  n.Status,
		Message: "notification accepted",
	})
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/notifications", h.CreateNotification)
}
```

- [ ] **Step 3: 验证代码编译通过**

```bash
go build ./internal/api
```

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler.go
git commit -m "feat: implement HTTP API handler"
```

---

### Task 6: 实现 main.go 入口

**Files:**
- Create: `main.go`

- [ ] **Step 1: 编写 main.go**

```go
package main

import (
	"log"
	"net/http"

	"code.byted.org/fintech_cf/common-notify/internal/api"
	"code.byted.org/fintech_cf/common-notify/internal/store"
	"code.byted.org/fintech_cf/common-notify/internal/worker"
)

func main() {
	s, err := store.NewSQLiteStore("./notifications.db")
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	w := worker.NewWorker(s, 3, 1000)
	w.Start()

	h := api.NewHandler(s, w)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	log.Println("server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
```

- [ ] **Step 2: 验证编译通过**

```bash
go build -o common-notify .
```

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat: add main.go entry point"
```

---

### Task 7: 测试端到端功能

**Files:**
- 无需创建新文件

- [ ] **Step 1: 启动服务（后台）**

```bash
./common-notify &
```

- [ ] **Step 2: 发送测试请求**

```bash
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

预期响应：
```json
{
  "id": 1,
  "status": "pending",
  "message": "notification accepted"
}
```

- [ ] **Step 3: 测试幂等性（发送相同 out_biz_no）**

```bash
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

预期响应（200 OK，显示已存在）：
```json
{
  "id": 1,
  "status": "success",
  "message": "notification already exists (idempotent)"
}
```

- [ ] **Step 4: 停止服务**

```bash
pkill -f common-notify
```

- [ ] **Step 5: Commit（如果有修改）**

---

### Task 8: 编写 AI 使用说明

**Files:**
- Create: `AI_USAGE.md`

- [ ] **Step 1: 编写 AI 使用说明**

```markdown
# AI 使用说明

## AI 在哪些关键地方提供了帮助

1. **需求理解与设计** - 帮助梳理核心问题、设计理念、系统边界
2. **架构设计** - 提出多方案对比，协助选择 MVP 方案
3. **数据模型设计** - 定义表结构、索引、状态机
4. **详细实施计划** - 分解任务到可执行的步骤，包含完整代码示例
5. **设计文档撰写** - 按照要求回答系统边界、可靠性、取舍等问题

## AI 给出过但未采纳的建议

| 未采纳的建议 | 原因 |
|-------------|------|
| 引入 Redis/RabbitMQ | MVP 阶段不需要，SQLite 已足够，减少外部依赖 |
| 分布式架构设计 | 超出 MVP 范围，单点足够 |
| 完善的监控告警体系 | 需求未要求，MVP 阶段简化 |
| 消息顺序保证 | 实现复杂，业务场景不要求 |

## 关键决策及原因

| 决策 | 原因 |
|-----|------|
| 使用 Go 语言 | 项目目录在 go/src 下，用户有 Go 背景，goroutine 适合 worker 场景 |
| 使用 SQLite | 无外部依赖，部署简单，MVP 读写量不大，事务支持 |
| 至少一次投递语义 | 业务场景宁愿重复也不丢失，外部系统通过 out_biz_no 做幂等 |
| 指数退避重试 | 平衡重试频率和系统负载 |
| 最大重试 5 次后进入死信 | 避免无限重试，人工介入更可控 |
```

- [ ] **Step 2: Commit**

```bash
git add AI_USAGE.md
git commit -m "docs: add AI usage documentation"
```

---

### Task 9: 更新 README.md

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 更新 README**

```markdown
# common-notify

外部 HTTP 通知投递系统 - 接收业务系统提交的外部 HTTP 通知请求，并可靠地投递到目标地址。

## 架构

- HTTP API 层：接收通知请求，幂等去重
- SQLite 存储：持久化通知任务
- Worker 池：并发投递，指数退避重试

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

## 设计文档

详见 [docs/superpowers/specs/2026-04-02-common-notify-design.md](docs/superpowers/specs/2026-04-02-common-notify-design.md)
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update README with project info"
```

---

## 计划自我检查

**1. Spec 覆盖：**
- ✅ 数据模型（out_biz_no, event_type）- Task 2
- ✅ SQLite 存储层 - Task 3
- ✅ Worker + 指数退避重试 - Task 4
- ✅ HTTP API + 幂等处理 - Task 5
- ✅ 系统边界明确 - 设计文档
- ✅ 可靠性说明 - 设计文档
- ✅ 取舍说明 - 设计文档

**2. Placeholder 扫描：** 无 TBD，所有步骤都有完整代码和命令

**3. 类型一致性：** 所有类型、方法名在各任务中一致

---

## 执行选项

Plan complete and saved to `docs/superpowers/plans/2026-04-02-common-notify-implementation.md`.

Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
