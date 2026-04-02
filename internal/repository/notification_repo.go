package repository

import (
	"database/sql"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/model"
)

// NotificationRepository 通知仓库
type NotificationRepository struct {
	db *DB
}

// NewNotificationRepository 创建通知仓库
func NewNotificationRepository(db *DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Create 创建通知
func (r *NotificationRepository) Create(n *model.Notification) error {
	now := time.Now()
	n.CreatedAt = now
	n.UpdatedAt = now
	if n.NextSendAt.IsZero() {
		n.NextSendAt = now
	}
	if n.Status == "" {
		n.Status = model.StatusPending
	}
	if n.MaxRetry == 0 {
		n.MaxRetry = 5
	}
	if n.RetryInterval == 0 {
		n.RetryInterval = 60 // 默认 60 秒
	}

	result, err := r.db.Exec(`
		INSERT INTO notifications (provider_id, idempotency_key, event_type, url, method, headers, body, status, retry_count, max_retry, retry_interval, next_send_at, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, n.ProviderID, n.IdempotencyKey, n.EventType, n.URL, n.Method, n.Headers, n.Body, n.Status, n.RetryCount, n.MaxRetry, n.RetryInterval, n.NextSendAt, n.LastError, n.CreatedAt, n.UpdatedAt)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	n.ID = id
	return nil
}

// GetByID 根据 ID 获取通知
func (r *NotificationRepository) GetByID(id int64) (*model.Notification, error) {
	var n model.Notification
	err := r.db.QueryRow(`
		SELECT id, provider_id, idempotency_key, event_type, url, method, headers, body, status, retry_count, max_retry, retry_interval, next_send_at, last_error, created_at, updated_at
		FROM notifications WHERE id = ?
	`, id).Scan(&n.ID, &n.ProviderID, &n.IdempotencyKey, &n.EventType, &n.URL, &n.Method, &n.Headers, &n.Body, &n.Status, &n.RetryCount, &n.MaxRetry, &n.RetryInterval, &n.NextSendAt, &n.LastError, &n.CreatedAt, &n.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// GetByIdempotencyKey 根据幂等键获取通知
func (r *NotificationRepository) GetByIdempotencyKey(key string) (*model.Notification, error) {
	if key == "" {
		return nil, nil
	}
	var n model.Notification
	err := r.db.QueryRow(`
		SELECT id, provider_id, idempotency_key, event_type, url, method, headers, body, status, retry_count, max_retry, retry_interval, next_send_at, last_error, created_at, updated_at
		FROM notifications WHERE idempotency_key = ?
	`, key).Scan(&n.ID, &n.ProviderID, &n.IdempotencyKey, &n.EventType, &n.URL, &n.Method, &n.Headers, &n.Body, &n.Status, &n.RetryCount, &n.MaxRetry, &n.RetryInterval, &n.NextSendAt, &n.LastError, &n.CreatedAt, &n.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// ListPending 获取待发送的通知
func (r *NotificationRepository) ListPending(limit int) ([]*model.Notification, error) {
	now := time.Now()
	rows, err := r.db.Query(`
		SELECT id, provider_id, idempotency_key, event_type, url, method, headers, body, status, retry_count, max_retry, retry_interval, next_send_at, last_error, created_at, updated_at
		FROM notifications
		WHERE status IN ('pending', 'failed') AND next_send_at <= ?
		ORDER BY next_send_at ASC
		LIMIT ?
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []*model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.ProviderID, &n.IdempotencyKey, &n.EventType, &n.URL, &n.Method, &n.Headers, &n.Body, &n.Status, &n.RetryCount, &n.MaxRetry, &n.RetryInterval, &n.NextSendAt, &n.LastError, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, &n)
	}
	return notifications, rows.Err()
}

// ListByStatus 根据状态获取通知
func (r *NotificationRepository) ListByStatus(status model.NotificationStatus, limit, offset int) ([]*model.Notification, error) {
	rows, err := r.db.Query(`
		SELECT id, provider_id, idempotency_key, event_type, url, method, headers, body, status, retry_count, max_retry, retry_interval, next_send_at, last_error, created_at, updated_at
		FROM notifications
		WHERE status = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []*model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.ProviderID, &n.IdempotencyKey, &n.EventType, &n.URL, &n.Method, &n.Headers, &n.Body, &n.Status, &n.RetryCount, &n.MaxRetry, &n.RetryInterval, &n.NextSendAt, &n.LastError, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, &n)
	}
	return notifications, rows.Err()
}

// Update 更新通知
func (r *NotificationRepository) Update(n *model.Notification) error {
	n.UpdatedAt = time.Now()
	_, err := r.db.Exec(`
		UPDATE notifications
		SET provider_id = ?, idempotency_key = ?, event_type = ?, url = ?, method = ?, headers = ?, body = ?, status = ?, retry_count = ?, max_retry = ?, retry_interval = ?, next_send_at = ?, last_error = ?, updated_at = ?
		WHERE id = ?
	`, n.ProviderID, n.IdempotencyKey, n.EventType, n.URL, n.Method, n.Headers, n.Body, n.Status, n.RetryCount, n.MaxRetry, n.RetryInterval, n.NextSendAt, n.LastError, n.UpdatedAt, n.ID)
	return err
}

// UpdateStatus 更新通知状态
func (r *NotificationRepository) UpdateStatus(id int64, status model.NotificationStatus, lastError string) error {
	_, err := r.db.Exec(`
		UPDATE notifications
		SET status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, lastError, id)
	return err
}

// MarkForRetry 标记通知为重试
func (r *NotificationRepository) MarkForRetry(id int64, retryCount int, nextSendAt time.Time, lastError string) error {
	status := model.StatusPending
	_, err := r.db.Exec(`
		UPDATE notifications
		SET status = ?, retry_count = ?, next_send_at = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, retryCount, nextSendAt, lastError, id)
	return err
}

// MarkFailed 标记通知为最终失败
func (r *NotificationRepository) MarkFailed(id int64, lastError string) error {
	_, err := r.db.Exec(`
		UPDATE notifications
		SET status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, model.StatusFailed, lastError, id)
	return err
}
