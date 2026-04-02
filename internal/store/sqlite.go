// Package store 提供数据持久化存储层
// 使用 SQLite 作为存储引擎，保证通知任务不丢失
package store

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"code.byted.org/fintech_cf/common-notify/internal/model"
)

// schema 数据库表结构定义
// 创建 notifications 表和相关索引
const schema = `
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    out_biz_no TEXT UNIQUE,           -- 外部业务单号，唯一索引用于幂等
    event_type TEXT NOT NULL,          -- 事件类型
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL,              -- pending/processing/success/dead
    target_url TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'POST',
    headers TEXT,                       -- JSON 格式的请求头
    body TEXT,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 5,
    last_error TEXT,
    next_retry_at TIMESTAMP            -- 下次重试时间
);

-- 索引：加速按状态查询
CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);
-- 索引：加速按事件类型查询
CREATE INDEX IF NOT EXISTS idx_notifications_event_type ON notifications(event_type);
-- 索引：加速按下次重试时间查询
CREATE INDEX IF NOT EXISTS idx_notifications_next_retry_at ON notifications(next_retry_at);
`

// SQLiteStore SQLite 存储层实现
// 封装所有数据库操作
type SQLiteStore struct {
	db *sql.DB // 数据库连接
}

// NewSQLiteStore 创建一个新的 SQLite 存储实例
// dbPath: SQLite 数据库文件路径（如 "./notifications.db"）
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// 打开 SQLite 数据库连接
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// 初始化表结构和索引
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	return &SQLiteStore{db: db}, nil
}

// Close 关闭数据库连接
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// CreateNotification 创建一个新的通知任务
// req: 创建通知的请求参数
// 返回：创建的通知任务对象
func (s *SQLiteStore) CreateNotification(req *model.CreateNotificationRequest) (*model.Notification, error) {
	// 将 Headers map 序列化为 JSON 字符串
	headersJSON, err := json.Marshal(req.Headers)
	if err != nil {
		return nil, err
	}

	// 默认使用 POST 方法
	if req.Method == "" {
		req.Method = "POST"
	}

	// 插入数据库
	result, err := s.db.Exec(`
		INSERT INTO notifications (out_biz_no, event_type, status, target_url, method, headers, body, max_retries)
		VALUES (?, ?, ?, ?, ?, ?, ?, 5)
	`, req.OutBizNo, req.EventType, model.StatusPending, req.TargetURL, req.Method, string(headersJSON), req.Body)
	if err != nil {
		return nil, err
	}

	// 获取新创建的记录 ID
	id, _ := result.LastInsertId()
	return s.GetNotificationByID(id)
}

// GetNotificationByOutBizNo 通过外部业务单号查询通知任务
// outBizNo: 外部业务单号
// 返回：通知任务对象，如果不存在返回 nil
func (s *SQLiteStore) GetNotificationByOutBizNo(outBizNo string) (*model.Notification, error) {
	row := s.db.QueryRow(`
		SELECT id, out_biz_no, event_type, created_at, status, target_url, method, headers, body, retry_count, max_retries, last_error, next_retry_at
		FROM notifications WHERE out_biz_no = ?
	`, outBizNo)
	return scanNotification(row)
}

// GetNotificationByID 通过 ID 查询通知任务
// id: 通知任务 ID
// 返回：通知任务对象
func (s *SQLiteStore) GetNotificationByID(id int64) (*model.Notification, error) {
	row := s.db.QueryRow(`
		SELECT id, out_biz_no, event_type, created_at, status, target_url, method, headers, body, retry_count, max_retries, last_error, next_retry_at
		FROM notifications WHERE id = ?
	`, id)
	return scanNotification(row)
}

// ListPendingNotifications 获取所有待处理的通知任务
// limit: 返回结果的最大数量
// 返回：待处理的通知任务列表，按创建时间升序排列
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

// UpdateStatus 更新通知任务的状态
// id: 通知任务 ID
// status: 新状态
func (s *SQLiteStore) UpdateStatus(id int64, status model.NotificationStatus) error {
	_, err := s.db.Exec(`UPDATE notifications SET status = ? WHERE id = ?`, status, id)
	return err
}

// MarkSuccess 标记通知任务为成功状态
// id: 通知任务 ID
func (s *SQLiteStore) MarkSuccess(id int64) error {
	_, err := s.db.Exec(`UPDATE notifications SET status = ? WHERE id = ?`, model.StatusSuccess, id)
	return err
}

// MarkDead 标记通知任务为死信状态
// id: 通知任务 ID
// lastError: 最后一次失败的错误信息
func (s *SQLiteStore) MarkDead(id int64, lastError string) error {
	_, err := s.db.Exec(`UPDATE notifications SET status = ?, last_error = ? WHERE id = ?`, model.StatusDead, lastError, id)
	return err
}

// ScheduleRetry 安排通知任务的下次重试
// id: 通知任务 ID
// retryCount: 当前重试次数
// nextRetryAt: 下次重试时间
// lastError: 最后一次失败的错误信息
func (s *SQLiteStore) ScheduleRetry(id int64, retryCount int, nextRetryAt time.Time, lastError string) error {
	_, err := s.db.Exec(`
		UPDATE notifications
		SET status = ?, retry_count = ?, next_retry_at = ?, last_error = ?
		WHERE id = ?
	`, model.StatusPending, retryCount, nextRetryAt, lastError, id)
	return err
}

// scanNotification 从单行结果扫描 Notification 对象
// row: SQL 查询单行结果
// 返回：Notification 对象
func scanNotification(row *sql.Row) (*model.Notification, error) {
	var n model.Notification
	var nextRetryAt sql.NullTime
	var lastError sql.NullString
	var headers sql.NullString
	var body sql.NullString
	var outBizNo sql.NullString
	err := row.Scan(
		&n.ID, &outBizNo, &n.EventType, &n.CreatedAt, &n.Status,
		&n.TargetURL, &n.Method, &headers, &body, &n.RetryCount,
		&n.MaxRetries, &lastError, &nextRetryAt,
	)
	if err != nil {
		return nil, err
	}
	// 处理可空字段
	if outBizNo.Valid {
		n.OutBizNo = outBizNo.String
	}
	if headers.Valid {
		n.Headers = headers.String
	}
	if body.Valid {
		n.Body = body.String
	}
	if lastError.Valid {
		n.LastError = lastError.String
	}
	if nextRetryAt.Valid {
		n.NextRetryAt = &nextRetryAt.Time
	}
	return &n, nil
}

// scanNotifications 从多行结果扫描 Notification 对象
// rows: SQL 查询多行结果
// 返回：Notification 对象
func scanNotifications(rows *sql.Rows) (*model.Notification, error) {
	var n model.Notification
	var nextRetryAt sql.NullTime
	var lastError sql.NullString
	var headers sql.NullString
	var body sql.NullString
	var outBizNo sql.NullString
	err := rows.Scan(
		&n.ID, &outBizNo, &n.EventType, &n.CreatedAt, &n.Status,
		&n.TargetURL, &n.Method, &headers, &body, &n.RetryCount,
		&n.MaxRetries, &lastError, &nextRetryAt,
	)
	if err != nil {
		return nil, err
	}
	// 处理可空字段
	if outBizNo.Valid {
		n.OutBizNo = outBizNo.String
	}
	if headers.Valid {
		n.Headers = headers.String
	}
	if body.Valid {
		n.Body = body.String
	}
	if lastError.Valid {
		n.LastError = lastError.String
	}
	if nextRetryAt.Valid {
		n.NextRetryAt = &nextRetryAt.Time
	}
	return &n, nil
}
