package service

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
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
