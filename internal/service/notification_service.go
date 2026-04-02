package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/repository"
)

// NotificationService 通知服务
type NotificationService struct {
	notificationRepo *repository.NotificationRepository
	providerRepo     *repository.ProviderRepository
	httpClient       *http.Client
}

// NewNotificationService 创建通知服务
func NewNotificationService(notificationRepo *repository.NotificationRepository, providerRepo *repository.ProviderRepository) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		providerRepo:     providerRepo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateNotification 创建通知
func (s *NotificationService) CreateNotification(n *model.Notification) error {
	// 验证必填字段
	if n.ProviderID == 0 {
		return errors.New("provider_id is required")
	}

	// 幂等性检查：如果提供了幂等键，先检查是否已存在
	if n.IdempotencyKey != "" {
		existing, err := s.notificationRepo.GetByIdempotencyKey(n.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			// 已存在，返回已有的通知（不重复创建）
			*n = *existing
			return nil
		}
	}

	// 验证供应商是否存在
	provider, err := s.providerRepo.GetByID(n.ProviderID)
	if err != nil {
		return err
	}
	if provider == nil {
		return errors.New("provider not found")
	}

	// 合并供应商配置
	if n.Method == "" {
		n.Method = provider.Method
	}
	if n.URL == "" {
		n.URL = provider.BaseURL
	}
	if n.Headers == nil {
		n.Headers = make(model.StringMap)
	}
	// 合并 headers（通知的 headers 覆盖供应商的）
	for k, v := range provider.Headers {
		if _, exists := n.Headers[k]; !exists {
			n.Headers[k] = v
		}
	}

	return s.notificationRepo.Create(n)
}

// GetNotification 获取通知
func (s *NotificationService) GetNotification(id int64) (*model.Notification, error) {
	return s.notificationRepo.GetByID(id)
}

// ListNotificationsByStatus 根据状态获取通知
func (s *NotificationService) ListNotificationsByStatus(status model.NotificationStatus, limit, offset int) ([]*model.Notification, error) {
	return s.notificationRepo.ListByStatus(status, limit, offset)
}

// ProcessPendingNotifications 处理待发送通知
func (s *NotificationService) ProcessPendingNotifications(limit int) int {
	notifications, err := s.notificationRepo.ListPending(limit)
	if err != nil {
		log.Printf("Failed to list pending notifications: %v", err)
		return 0
	}

	processed := 0
	for _, n := range notifications {
		if err := s.processNotification(n); err != nil {
			log.Printf("Failed to process notification %d: %v", n.ID, err)
		} else {
			processed++
		}
	}
	return processed
}

// processNotification 处理单个通知
func (s *NotificationService) processNotification(n *model.Notification) error {
	// 标记为发送中
	if err := s.notificationRepo.UpdateStatus(n.ID, model.StatusSending, ""); err != nil {
		return err
	}

	// 执行 HTTP 请求
	err := s.sendHTTPRequest(n)
	if err == nil {
		// 发送成功
		return s.notificationRepo.UpdateStatus(n.ID, model.StatusSuccess, "")
	}

	// 发送失败，处理重试
	n.RetryCount++
	if n.RetryCount >= n.MaxRetry {
		// 达到最大重试次数，标记为最终失败
		return s.notificationRepo.MarkFailed(n.ID, err.Error())
	}

	// 标记为待重试
	nextSendAt := time.Now().Add(time.Duration(n.RetryInterval) * time.Second)
	return s.notificationRepo.MarkForRetry(n.ID, n.RetryCount, nextSendAt, err.Error())
}

// sendHTTPRequest 发送 HTTP 请求
func (s *NotificationService) sendHTTPRequest(n *model.Notification) error {
	var bodyReader io.Reader
	if n.Body != "" {
		bodyReader = bytes.NewReader([]byte(n.Body))
	}

	req, err := http.NewRequest(n.Method, n.URL, bodyReader)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	// 设置请求头
	for k, v := range n.Headers {
		req.Header.Set(k, v)
	}
	// 如果没有 Content-Type 且有 body，默认设置为 application/json
	if n.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体（忽略内容，只用于关闭连接）
	_, _ = io.Copy(io.Discard, resp.Body)

	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// RetryNotification 手动重试通知
func (s *NotificationService) RetryNotification(id int64) error {
	n, err := s.notificationRepo.GetByID(id)
	if err != nil {
		return err
	}
	if n == nil {
		return errors.New("notification not found")
	}

	// 重置状态
	n.Status = model.StatusPending
	n.RetryCount = 0
	n.NextSendAt = time.Now()
	n.LastError = ""

	return s.notificationRepo.Update(n)
}
