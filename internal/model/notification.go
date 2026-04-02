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
