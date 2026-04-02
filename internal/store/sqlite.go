package store

import (
	"database/sql"
	"encoding/json"
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

	_, err := s.db.Exec(`
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
