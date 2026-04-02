package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// NotificationStatus 通知状态
type NotificationStatus string

const (
	StatusPending NotificationStatus = "pending" // 待发送
	StatusSending NotificationStatus = "sending" // 发送中
	StatusSuccess NotificationStatus = "success" // 发送成功
	StatusFailed  NotificationStatus = "failed"  // 发送失败（已达最大重试次数）
)

// Provider 供应商配置
type Provider struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`         // 供应商名称
	BaseURL   string    `json:"base_url" db:"base_url"` // 基础 URL
	Method    string    `json:"method" db:"method"`     // HTTP 方法: GET, POST, PUT, DELETE
	Headers   StringMap `json:"headers" db:"headers"`   // 请求头
	BodyTpl   string    `json:"body_tpl" db:"body_tpl"` // 请求体模板（可选）
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Notification 通知任务
type Notification struct {
	ID             int64              `json:"id" db:"id"`
	ProviderID     int64              `json:"provider_id" db:"provider_id"`
	IdempotencyKey string             `json:"idempotency_key" db:"idempotency_key"` // 幂等键，用于去重
	EventType      string             `json:"event_type" db:"event_type"`           // 事件类型
	URL            string             `json:"url" db:"url"`                         // 完整请求 URL（如果不为空，覆盖 provider 的 base_url）
	Method         string             `json:"method" db:"method"`                   // HTTP 方法（如果不为空，覆盖 provider 的 method）
	Headers        StringMap          `json:"headers" db:"headers"`                 // 请求头（合并 provider 的 headers）
	Body           string             `json:"body" db:"body"`                       // 请求体
	Status         NotificationStatus `json:"status" db:"status"`
	RetryCount     int                `json:"retry_count" db:"retry_count"`       // 已重试次数
	MaxRetry       int                `json:"max_retry" db:"max_retry"`           // 最大重试次数
	RetryInterval  int                `json:"retry_interval" db:"retry_interval"` // 重试间隔（秒）
	NextSendAt     time.Time          `json:"next_send_at" db:"next_send_at"`     // 下次发送时间
	LastError      string             `json:"last_error" db:"last_error"`         // 上次错误信息
	CreatedAt      time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at" db:"updated_at"`
}

// StringMap 自定义类型，用于存储 map[string]string 到数据库
type StringMap map[string]string

// Value 实现 driver.Valuer 接口
func (m StringMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	return json.Marshal(m)
}

// Scan 实现 sql.Scanner 接口
func (m *StringMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(StringMap)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("type assertion to []byte or string failed")
	}
	return json.Unmarshal(bytes, m)
}
