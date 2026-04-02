# Common-Notify 通知服务实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现一个可靠的 HTTP 通知服务，支持多供应商配置、幂等性保证、自动重试机制。

**Architecture:** 单体应用，分为 API 层、服务层、数据层和定时任务层。使用 Gin 提供 HTTP API，SQLite 持久化存储，cron 定时任务驱动重试。

**Tech Stack:** Go, Gin, SQLite (mattn/go-sqlite3), robfig/cron

---

## 文件结构规划

```
common-notify/
├── go.mod                          # Go 模块定义
├── go.sum                          # 依赖锁定
├── main.go                         # 程序入口
├── internal/
│   ├── model/
│   │   └── model.go               # 数据模型定义
│   ├── store/
│   │   └── sqlite.go              # SQLite 存储实现
│   ├── api/
│   │   └── handler.go             # HTTP 处理器
│   ├── service/
│   │   ├── notification.go        # 通知服务
│   │   └── vendor.go              # 供应商服务
│   └── worker/
│       └── worker.go              # 定时任务 worker
└── AI_USAGE.md                     # AI 使用说明
```

---

## 任务列表

### Task 1: 初始化 Go 项目和依赖

**Files:**
- Create: `go.mod`
- Create: `go.sum`

- [ ] **Step 1: 初始化 Go 模块**

```bash
go mod init code.byted.org/fintech_cf/common-notify
```

- [ ] **Step 2: 添加依赖**

```bash
go get github.com/gin-gonic/gin
go get github.com/mattn/go-sqlite3
go get github.com/robfig/cron/v3
```

- [ ] **Step 3: 验证依赖**

```bash
go mod tidy
```

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "feat: initialize Go module and add dependencies"
```

---

### Task 2: 定义数据模型

**Files:**
- Create: `internal/model/model.go`

- [ ] **Step 1: 创建模型文件**

```go
package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Notification 通知记录
type Notification struct {
	ID            int64        `json:"id" db:"id"`
	OutBizNo      string       `json:"out_biz_no" db:"out_biz_no"`
	VendorID      string       `json:"vendor_id" db:"vendor_id"`
	EventType     string       `json:"event_type" db:"event_type"`
	Payload       JSONMap      `json:"payload" db:"payload"`
	Status        string       `json:"status" db:"status"`
	Attempts      int          `json:"attempts" db:"attempts"`
	LastAttemptAt *time.Time   `json:"last_attempt_at" db:"last_attempt_at"`
	NextAttemptAt time.Time    `json:"next_attempt_at" db:"next_attempt_at"`
	ErrorMessage  string       `json:"error_message" db:"error_message"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
}

// Vendor 供应商配置
type Vendor struct {
	ID            string       `json:"id" db:"id"`
	Name          string       `json:"name" db:"name"`
	EndpointURL   string       `json:"endpoint_url" db:"endpoint_url"`
	Method        string       `json:"method" db:"method"`
	Headers       JSONMap      `json:"headers" db:"headers"`
	BodyTemplate  string       `json:"body_template" db:"body_template"`
	TimeoutMs     int          `json:"timeout_ms" db:"timeout_ms"`
	SkipTLSVerify bool         `json:"skip_tls_verify" db:"skip_tls_verify"`
	CACert        string       `json:"ca_cert" db:"ca_cert"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
}

// JSONMap 自定义 JSON 类型
type JSONMap map[string]interface{}

// Value 实现 driver.Valuer 接口
func (j JSONMap) Value() (driver.Value, error) {
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
func (j *JSONMap) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

// 状态常量
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusSuccess    = "success"
	StatusFailed     = "failed"
)

// 通知状态是否终态
func (n *Notification) IsTerminal() bool {
	return n.Status == StatusSuccess || n.Status == StatusFailed
}
```

- [ ] **Step 2: Commit**

```bash
mkdir -p internal/model
git add internal/model/model.go
git commit -m "feat: add data models"
```

---

### Task 3: 实现 SQLite 存储层

**Files:**
- Create: `internal/store/sqlite.go`

- [ ] **Step 1: 创建存储层文件**

```go
package store

import (
	"database/sql"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	_ "github.com/mattn/go-sqlite3"
)

// Store SQLite 存储实现
type Store struct {
	db *sql.DB
}

// NewStore 创建新的存储实例
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		return nil, err
	}

	return s, nil
}

// 初始化数据库表
func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS vendors (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		endpoint_url TEXT NOT NULL,
		method TEXT NOT NULL DEFAULT 'POST',
		headers TEXT,
		body_template TEXT,
		timeout_ms INTEGER NOT NULL DEFAULT 5000,
		skip_tls_verify BOOLEAN NOT NULL DEFAULT 0,
		ca_cert TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS notifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		out_biz_no TEXT NOT NULL,
		vendor_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		payload TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		attempts INTEGER NOT NULL DEFAULT 0,
		last_attempt_at DATETIME,
		next_attempt_at DATETIME NOT NULL,
		error_message TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(out_biz_no, vendor_id)
	);

	CREATE INDEX IF NOT EXISTS idx_notifications_status_next ON notifications(status, next_attempt_at);
	CREATE INDEX IF NOT EXISTS idx_notifications_vendor ON notifications(vendor_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close 关闭数据库连接
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateVendor 创建供应商配置
func (s *Store) CreateVendor(vendor *model.Vendor) error {
	now := time.Now()
	vendor.CreatedAt = now
	vendor.UpdatedAt = now

	headersJSON, err := json.Marshal(vendor.Headers)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO vendors (id, name, endpoint_url, method, headers, body_template, timeout_ms, skip_tls_verify, ca_cert, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, vendor.ID, vendor.Name, vendor.EndpointURL, vendor.Method, headersJSON, vendor.BodyTemplate, vendor.TimeoutMs, vendor.SkipTLSVerify, vendor.CACert, vendor.CreatedAt, vendor.UpdatedAt)

	return err
}

// GetVendor 获取供应商配置
func (s *Store) GetVendor(id string) (*model.Vendor, error) {
	var vendor model.Vendor
	var headersJSON []byte

	err := s.db.QueryRow(`
		SELECT id, name, endpoint_url, method, headers, body_template, timeout_ms, skip_tls_verify, ca_cert, created_at, updated_at
		FROM vendors WHERE id = ?
	`, id).Scan(&vendor.ID, &vendor.Name, &vendor.EndpointURL, &vendor.Method, &headersJSON, &vendor.BodyTemplate, &vendor.TimeoutMs, &vendor.SkipTLSVerify, &vendor.CACert, &vendor.CreatedAt, &vendor.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if len(headersJSON) > 0 {
		if err := json.Unmarshal(headersJSON, &vendor.Headers); err != nil {
			return nil, err
		}
	}

	return &vendor, nil
}

// CreateNotification 创建通知记录（幂等）
func (s *Store) CreateNotification(notification *model.Notification) (*model.Notification, error) {
	now := time.Now()
	notification.CreatedAt = now
	notification.UpdatedAt = now
	notification.Status = model.StatusPending
	notification.NextAttemptAt = now

	payloadJSON, err := json.Marshal(notification.Payload)
	if err != nil {
		return nil, err
	}

	// 先尝试查找已存在的记录
	var existingID int64
	err = s.db.QueryRow(`
		SELECT id FROM notifications WHERE out_biz_no = ? AND vendor_id = ?
	`, notification.OutBizNo, notification.VendorID).Scan(&existingID)

	if err == nil {
		// 已存在，返回已有记录
		return s.GetNotification(existingID)
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	// 不存在，插入新记录
	result, err := s.db.Exec(`
		INSERT INTO notifications (out_biz_no, vendor_id, event_type, payload, status, next_attempt_at, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, notification.OutBizNo, notification.VendorID, notification.EventType, payloadJSON, notification.Status, notification.NextAttemptAt, notification.ErrorMessage, notification.CreatedAt, notification.UpdatedAt)

	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return s.GetNotification(id)
}

// GetNotification 获取通知记录
func (s *Store) GetNotification(id int64) (*model.Notification, error) {
	var notification model.Notification
	var payloadJSON []byte
	var lastAttemptAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, out_biz_no, vendor_id, event_type, payload, status, attempts, last_attempt_at, next_attempt_at, error_message, created_at, updated_at
		FROM notifications WHERE id = ?
	`, id).Scan(&notification.ID, &notification.OutBizNo, &notification.VendorID, &notification.EventType, &payloadJSON, &notification.Status, &notification.Attempts, &lastAttemptAt, &notification.NextAttemptAt, &notification.ErrorMessage, &notification.CreatedAt, &notification.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if lastAttemptAt.Valid {
		notification.LastAttemptAt = &lastAttemptAt.Time
	}

	if err := json.Unmarshal(payloadJSON, &notification.Payload); err != nil {
		return nil, err
	}

	return &notification, nil
}

// GetPendingNotifications 获取待发送的通知
func (s *Store) GetPendingNotifications(limit int) ([]*model.Notification, error) {
	now := time.Now()
	rows, err := s.db.Query(`
		SELECT id, out_biz_no, vendor_id, event_type, payload, status, attempts, last_attempt_at, next_attempt_at, error_message, created_at, updated_at
		FROM notifications
		WHERE status IN (?, ?) AND next_attempt_at <= ?
		ORDER BY next_attempt_at ASC
		LIMIT ?
	`, model.StatusPending, model.StatusProcessing, now, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []*model.Notification
	for rows.Next() {
		var notification model.Notification
		var payloadJSON []byte
		var lastAttemptAt sql.NullTime

		err := rows.Scan(&notification.ID, &notification.OutBizNo, &notification.VendorID, &notification.EventType, &payloadJSON, &notification.Status, &notification.Attempts, &lastAttemptAt, &notification.NextAttemptAt, &notification.ErrorMessage, &notification.CreatedAt, &notification.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if lastAttemptAt.Valid {
			notification.LastAttemptAt = &lastAttemptAt.Time
		}

		if err := json.Unmarshal(payloadJSON, &notification.Payload); err != nil {
			return nil, err
		}

		notifications = append(notifications, &notification)
	}

	return notifications, nil
}

// UpdateNotification 更新通知记录
func (s *Store) UpdateNotification(notification *model.Notification) error {
	now := time.Now()
	notification.UpdatedAt = now

	payloadJSON, err := json.Marshal(notification.Payload)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		UPDATE notifications
		SET status = ?, attempts = ?, last_attempt_at = ?, next_attempt_at = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`, notification.Status, notification.Attempts, notification.LastAttemptAt, notification.NextAttemptAt, notification.ErrorMessage, notification.UpdatedAt, notification.ID)

	return err
}

// ResetNotification 重置通知状态用于重试
func (s *Store) ResetNotification(id int64) error {
	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE notifications
		SET status = ?, attempts = 0, last_attempt_at = NULL, next_attempt_at = ?, error_message = '', updated_at = ?
		WHERE id = ?
	`, model.StatusPending, now, now, id)

	return err
}
```

- [ ] **Step 2: 添加缺失的 import**

在文件顶部添加 `"encoding/json"` import。

- [ ] **Step 3: Commit**

```bash
mkdir -p internal/store
git add internal/store/sqlite.go
git commit -m "feat: implement SQLite storage layer"
```

---

### Task 4: 实现服务层

**Files:**
- Create: `internal/service/notification.go`
- Create: `internal/service/vendor.go`

- [ ] **Step 1: 创建供应商服务**

```go
package service

import (
	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/store"
)

// VendorService 供应商服务
type VendorService struct {
	store *store.Store
}

// NewVendorService 创建供应商服务
func NewVendorService(s *store.Store) *VendorService {
	return &VendorService{store: s}
}

// CreateVendor 创建供应商
func (s *VendorService) CreateVendor(vendor *model.Vendor) error {
	// 设置默认值
	if vendor.Method == "" {
		vendor.Method = "POST"
	}
	if vendor.TimeoutMs == 0 {
		vendor.TimeoutMs = 5000
	}
	return s.store.CreateVendor(vendor)
}

// GetVendor 获取供应商
func (s *VendorService) GetVendor(id string) (*model.Vendor, error) {
	return s.store.GetVendor(id)
}
```

- [ ] **Step 2: 创建通知服务**

```go
package service

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/store"
)

// NotificationService 通知服务
type NotificationService struct {
	store         *store.Store
	vendorService *VendorService
}

// NewNotificationService 创建通知服务
func NewNotificationService(s *store.Store, vs *VendorService) *NotificationService {
	return &NotificationService{store: s, vendorService: vs}
}

// CreateNotification 创建通知
func (s *NotificationService) CreateNotification(notification *model.Notification) (*model.Notification, error) {
	return s.store.CreateNotification(notification)
}

// GetNotification 获取通知
func (s *NotificationService) GetNotification(id int64) (*model.Notification, error) {
	return s.store.GetNotification(id)
}

// RetryNotification 重试通知
func (s *NotificationService) RetryNotification(id int64) error {
	return s.store.ResetNotification(id)
}

// ProcessPendingNotifications 处理待发送通知
func (s *NotificationService) ProcessPendingNotifications(limit int) error {
	notifications, err := s.store.GetPendingNotifications(limit)
	if err != nil {
		return err
	}

	for _, notification := range notifications {
		if err := s.processNotification(notification); err != nil {
			// 记录错误但继续处理其他通知
			continue
		}
	}

	return nil
}

// 处理单个通知
func (s *NotificationService) processNotification(notification *model.Notification) error {
	vendor, err := s.vendorService.GetVendor(notification.VendorID)
	if err != nil {
		return s.markFailed(notification, fmt.Sprintf("failed to get vendor: %v", err))
	}

	// 标记为处理中
	notification.Status = model.StatusProcessing
	if err := s.store.UpdateNotification(notification); err != nil {
		return err
	}

	// 执行发送
	err = s.sendNotification(notification, vendor)
	now := time.Now()
	notification.LastAttemptAt = &now
	notification.Attempts++

	if err != nil {
		// 发送失败
		notification.ErrorMessage = err.Error()
		maxAttempts := 6
		if notification.Attempts >= maxAttempts {
			return s.markFailed(notification, err.Error())
		}
		// 计算下次重试时间：指数退避
		backoff := time.Duration(1<<notification.Attempts) * time.Second
		notification.NextAttemptAt = now.Add(backoff)
		notification.Status = model.StatusPending
		return s.store.UpdateNotification(notification)
	}

	// 发送成功
	return s.markSuccess(notification)
}

// 发送 HTTP 请求
func (s *NotificationService) sendNotification(notification *model.Notification, vendor *model.Vendor) error {
	// 构建请求体
	body, err := s.buildBody(notification, vendor)
	if err != nil {
		return err
	}

	// 创建 HTTP 客户端
	client, err := s.createHTTPClient(vendor)
	if err != nil {
		return err
	}

	// 创建请求
	req, err := http.NewRequest(vendor.Method, vendor.EndpointURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	// 设置请求头
	for k, v := range vendor.Headers {
		if strVal, ok := v.(string); ok {
			req.Header.Set(k, strVal)
		}
	}
	req.Header.Set("Content-Type", "application/json")

	// 执行请求
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == 429 {
		return fmt.Errorf("rate limited: status %d", resp.StatusCode)
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return fmt.Errorf("client error: status %d", resp.StatusCode)
	}

	return fmt.Errorf("server error: status %d", resp.StatusCode)
}

// 构建请求体
func (s *NotificationService) buildBody(notification *model.Notification, vendor *model.Vendor) ([]byte, error) {
	if vendor.BodyTemplate == "" {
		// 没有模板，直接使用 payload
		return json.Marshal(notification.Payload)
	}

	// 使用模板渲染
	tmpl, err := template.New("body").Parse(vendor.BodyTemplate)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"event_type": notification.EventType,
		"payload":    notification.Payload,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// 创建 HTTP 客户端
func (s *NotificationService) createHTTPClient(vendor *model.Vendor) (*http.Client, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: vendor.SkipTLSVerify,
		},
	}

	// 如果配置了自定义 CA 证书
	if vendor.CACert != "" {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(vendor.CACert)) {
			return nil, fmt.Errorf("failed to parse CA cert")
		}
		transport.TLSClientConfig.RootCAs = caCertPool
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(vendor.TimeoutMs) * time.Millisecond,
	}

	return client, nil
}

// 标记为成功
func (s *NotificationService) markSuccess(notification *model.Notification) error {
	notification.Status = model.StatusSuccess
	notification.ErrorMessage = ""
	return s.store.UpdateNotification(notification)
}

// 标记为失败
func (s *NotificationService) markFailed(notification *model.Notification, errMsg string) error {
	notification.Status = model.StatusFailed
	notification.ErrorMessage = errMsg
	return s.store.UpdateNotification(notification)
}
```

- [ ] **Step 3: 给 notification.go 添加缺失的 import**

在文件顶部添加 `"encoding/json"` import。

- [ ] **Step 4: Commit**

```bash
mkdir -p internal/service
git add internal/service/notification.go internal/service/vendor.go
git commit -m "feat: implement service layer"
```

---

### Task 5: 实现 API 层

**Files:**
- Create: `internal/api/handler.go`

- [ ] **Step 1: 创建 API 处理器**

```go
package api

import (
	"net/http"
	"strconv"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler API 处理器
type Handler struct {
	notificationService *service.NotificationService
	vendorService       *service.VendorService
}

// NewHandler 创建 API 处理器
func NewHandler(ns *service.NotificationService, vs *service.VendorService) *Handler {
	return &Handler{
		notificationService: ns,
		vendorService:       vs,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// 供应商管理
	r.POST("/vendors", h.CreateVendor)
	r.GET("/vendors/:id", h.GetVendor)

	// 通知管理
	r.POST("/notifications", h.CreateNotification)
	r.GET("/notifications/:id", h.GetNotification)
	r.POST("/notifications/:id/retry", h.RetryNotification)
}

// CreateVendor 创建供应商
func (h *Handler) CreateVendor(c *gin.Context) {
	var vendor model.Vendor
	if err := c.ShouldBindJSON(&vendor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if vendor.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vendor id is required"})
		return
	}

	if err := h.vendorService.CreateVendor(&vendor); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, vendor)
}

// GetVendor 获取供应商
func (h *Handler) GetVendor(c *gin.Context) {
	id := c.Param("id")

	vendor, err := h.vendorService.GetVendor(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "vendor not found"})
		return
	}

	c.JSON(http.StatusOK, vendor)
}

// CreateNotification 创建通知
func (h *Handler) CreateNotification(c *gin.Context) {
	var req struct {
		OutBizNo  string                 `json:"out_biz_no" binding:"required"`
		VendorID  string                 `json:"vendor_id" binding:"required"`
		EventType string                 `json:"event_type" binding:"required"`
		Payload   map[string]interface{} `json:"payload" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	notification := &model.Notification{
		OutBizNo:  req.OutBizNo,
		VendorID:  req.VendorID,
		EventType: req.EventType,
		Payload:   req.Payload,
	}

	created, err := h.notificationService.CreateNotification(notification)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, created)
}

// GetNotification 获取通知
func (h *Handler) GetNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
		return
	}

	notification, err := h.notificationService.GetNotification(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "notification not found"})
		return
	}

	c.JSON(http.StatusOK, notification)
}

// RetryNotification 重试通知
func (h *Handler) RetryNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
		return
	}

	if err := h.notificationService.RetryNotification(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "notification queued for retry"})
}
```

- [ ] **Step 2: Commit**

```bash
mkdir -p internal/api
git add internal/api/handler.go
git commit -m "feat: implement API handlers"
```

---

### Task 6: 实现 Worker 定时任务

**Files:**
- Create: `internal/worker/worker.go`

- [ ] **Step 1: 创建 Worker**

```go
package worker

import (
	"log"

	"code.byted.org/fintech_cf/common-notify/internal/service"
	"github.com/robfig/cron/v3"
)

// Worker 定时任务 worker
type Worker struct {
	cron                *cron.Cron
	notificationService *service.NotificationService
}

// NewWorker 创建 worker
func NewWorker(ns *service.NotificationService) *Worker {
	return &Worker{
		cron:                cron.New(),
		notificationService: ns,
	}
}

// Start 启动 worker
func (w *Worker) Start() error {
	// 每 10 秒执行一次
	_, err := w.cron.AddFunc("*/10 * * * * *", func() {
		if err := w.notificationService.ProcessPendingNotifications(100); err != nil {
			log.Printf("error processing notifications: %v", err)
		}
	})

	if err != nil {
		return err
	}

	w.cron.Start()
	log.Println("worker started")
	return nil
}

// Stop 停止 worker
func (w *Worker) Stop() {
	ctx := w.cron.Stop()
	<-ctx.Done()
	log.Println("worker stopped")
}
```

- [ ] **Step 2: Commit**

```bash
mkdir -p internal/worker
git add internal/worker/worker.go
git commit -m "feat: implement cron worker"
```

---

### Task 7: 实现主程序入口

**Files:**
- Create: `main.go`

- [ ] **Step 1: 创建主程序**

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/api"
	"code.byted.org/fintech_cf/common-notify/internal/service"
	"code.byted.org/fintech_cf/common-notify/internal/store"
	"code.byted.org/fintech_cf/common-notify/internal/worker"
	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化存储
	dbPath := "./common-notify.db"
	s, err := store.NewStore(dbPath)
	if err != nil {
		log.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	// 初始化服务
	vendorService := service.NewVendorService(s)
	notificationService := service.NewNotificationService(s, vendorService)

	// 初始化 API
	handler := api.NewHandler(notificationService, vendorService)

	// 设置 Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	handler.RegisterRoutes(r)

	// 初始化并启动 worker
	w := worker.NewWorker(notificationService)
	if err := w.Start(); err != nil {
		log.Fatalf("failed to start worker: %v", err)
	}
	defer w.Stop()

	// 启动 HTTP 服务器
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		log.Println("server starting on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	log.Println("server exited")
}
```

- [ ] **Step 2: Commit**

```bash
git add main.go
git commit -m "feat: add main entrypoint"
```

---

### Task 8: 创建 AI 使用说明

**Files:**
- Create: `AI_USAGE.md`

- [ ] **Step 1: 创建 AI 使用说明**

```markdown
# AI 使用说明

## AI 在关键地方提供的帮助

### 1. 整体架构设计
- AI 帮助梳理了系统的边界，明确了哪些功能应该包含在 MVP 中，哪些应该后续迭代
- AI 提供了分层架构的建议（API 层、服务层、数据层、Worker 层），使得代码结构清晰

### 2. 数据模型设计
- AI 建议添加 `out_biz_no` 字段用于幂等性保证
- AI 建议添加 `event_type` 字段用于区分不同上游事件
- AI 设计了 `vendors` 表的 `skip_tls_verify` 和 `ca_cert` 字段，用于支持 HTTP 和 HTTPS 两种协议

### 3. 重试机制设计
- AI 建议使用指数退避策略（1s, 2s, 4s, 8s, 16s, 32s）
- AI 明确了投递语义为"至少一次"，并解释了为什么不选择"精确一次"

### 4. 代码实现指导
- AI 提供了完整的文件结构规划
- AI 为每个模块提供了详细的实现代码，包括错误处理
- AI 帮助设计了清晰的接口定义

## AI 给出过但没有采纳的建议

### 1. 使用消息队列（RabbitMQ/Kafka）
- **AI 建议**: 引入消息队列来实现更可靠的异步处理
- **未采纳原因**: 对于 MVP 来说过于复杂，SQLite + Cron 已经足够满足需求
- **未来演进**: 如果流量显著增长，可以考虑引入消息队列

### 2. 分布式锁和多实例支持
- **AI 建议**: 实现分布式锁以支持多实例部署
- **未采纳原因**: MVP 假设单实例部署，多实例支持可以后续添加
- **未来演进**: 当需要高可用时，添加分布式锁（如基于 Redis）

### 3. 死信队列
- **AI 建议**: 为永久失败的通知添加死信队列
- **未采纳原因**: MVP 中可以通过管理 API 手动重试，死信队列不是必需的
- **未来演进**: 可以后续添加死信队列用于自动化处理永久失败的通知

## 关键决策及原因

### 1. 选择 SQLite 作为持久化存储
- **决策**: 使用 SQLite 而不是 PostgreSQL 或 MySQL
- **原因**: 零配置、单文件、易部署，完全满足 MVP 需求；PostgreSQL 需要额外的部署和运维成本

### 2. 选择 Cron 定时任务作为重试机制
- **决策**: 使用 Cron 定时任务而不是内存队列 + 实时 Worker
- **原因**: 简单可靠，避免引入消息队列的复杂性；虽然延迟稍高（最多 10 秒），但对于通知场景是可接受的

### 3. 模板化供应商配置
- **决策**: 使用模板而不是硬编码每个供应商的逻辑
- **原因**: 灵活支持不同供应商的 API 格式，新增供应商无需修改代码

### 4. 幂等性设计
- **决策**: 通过 `out_biz_no` + `vendor_id` 唯一索引实现幂等
- **原因**: 实现简单且可靠，业务系统无需担心重复调用
```

- [ ] **Step 2: Commit**

```bash
git add AI_USAGE.md
git commit -m "docs: add AI usage documentation"
```

---

### Task 9: 更新 README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 更新 README**

```markdown
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
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update README with project documentation"
```

---

## 计划自检

### 1. 规范覆盖检查
- ✅ 问题理解与系统价值 - Task 9 (README)
- ✅ 系统边界 - 通过代码结构体现
- ✅ 可靠性与失败处理 - Task 4 (service layer)
- ✅ 取舍与演进 - Task 8 (AI_USAGE.md)
- ✅ HTTPS 支持 - Task 2, Task 3, Task 4
- ✅ 幂等性 - Task 2, Task 3
- ✅ 供应商配置管理 - Task 2, Task 3, Task 4, Task 5

### 2. 占位符扫描
- ✅ 无 TBD/TODO
- ✅ 所有代码块完整
- ✅ 所有命令具体

### 3. 类型一致性检查
- ✅ 模型定义一致
- ✅ 方法签名一致
- ✅ 文件名和路径一致

---

## 执行选项

计划已保存到 `docs/superpowers/plans/2026-04-02-common-notify-implementation.md`。两个执行选项：

**1. Subagent-Driven (推荐)** - 我为每个任务派发一个新的子代理，任务间进行审查，快速迭代

**2. Inline Execution** - 在当前会话中使用 executing-plans 执行任务，批量执行带检查点

选择哪种方式？
