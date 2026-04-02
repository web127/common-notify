package store

import (
	"database/sql"
	"encoding/json"
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
