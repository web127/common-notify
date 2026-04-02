// Package model 定义数据模型和类型
package model

import "time"

// NotificationStatus 通知任务的状态类型
type NotificationStatus string

const (
	// StatusPending 待处理状态 - 任务已创建，等待被 worker 处理
	StatusPending NotificationStatus = "pending"
	// StatusProcessing 处理中状态 - worker 正在投递通知
	StatusProcessing NotificationStatus = "processing"
	// StatusSuccess 成功状态 - 通知已成功投递到外部系统
	StatusSuccess NotificationStatus = "success"
	// StatusDead 死信状态 - 超过最大重试次数，不再自动重试，等待人工介入
	StatusDead NotificationStatus = "dead"
)

// Notification 通知任务数据模型
// 存储一个通知任务的完整信息，包括目标地址、请求内容、重试状态等
type Notification struct {
	// ID 自增主键，唯一标识一个通知任务
	ID int64 `json:"id"`
	// OutBizNo 外部业务单号，用于幂等去重（可选但推荐）
	OutBizNo string `json:"out_biz_no"`
	// EventType 事件类型，用于标识上游业务和事件（如 "user.registered"）
	EventType string `json:"event_type"`
	// CreatedAt 任务创建时间
	CreatedAt time.Time `json:"created_at"`
	// Status 当前任务状态
	Status NotificationStatus `json:"status"`
	// TargetURL 目标外部系统的 URL
	TargetURL string `json:"target_url"`
	// Method HTTP 请求方法（如 POST、GET、PUT 等）
	Method string `json:"method"`
	// Headers HTTP 请求头，JSON 字符串格式
	Headers string `json:"headers"`
	// Body HTTP 请求体
	Body string `json:"body"`
	// RetryCount 当前已重试次数
	RetryCount int `json:"retry_count"`
	// MaxRetries 最大重试次数
	MaxRetries int `json:"max_retries"`
	// LastError 最后一次失败的错误信息
	LastError string `json:"last_error"`
	// NextRetryAt 下次重试时间（仅在 pending 且有重试计划时有效）
	NextRetryAt *time.Time `json:"next_retry_at"`
}

// CreateNotificationRequest 创建通知请求的 API 结构
// 上游业务系统通过此结构调用 common-notify API
type CreateNotificationRequest struct {
	// OutBizNo 外部业务单号，可选但推荐，用于幂等去重
	OutBizNo string `json:"out_biz_no"`
	// EventType 事件类型，必填，标识上游业务和事件
	EventType string `json:"event_type"`
	// TargetURL 目标 URL，必填
	TargetURL string `json:"target_url"`
	// Method HTTP 方法，可选，默认为 POST
	Method string `json:"method"`
	// Headers HTTP 请求头，可选
	Headers map[string]string `json:"headers"`
	// Body HTTP 请求体，可选
	Body string `json:"body"`
}

// CreateNotificationResponse 创建通知响应的 API 结构
// 返回给上游业务系统的响应
type CreateNotificationResponse struct {
	// ID 创建的通知任务 ID
	ID int64 `json:"id"`
	// Status 当前任务状态
	Status NotificationStatus `json:"status"`
	// Message 提示信息
	Message string `json:"message"`
}
