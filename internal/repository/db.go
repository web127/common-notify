package repository

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB 数据库连接封装
type DB struct {
	*sql.DB
}

// NewDB 创建数据库连接
func NewDB(dsn string) (*DB, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// 初始化表
	if err := initSchema(db); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return &DB{db}, nil
}

// initSchema 初始化数据库表结构
func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS providers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		base_url TEXT NOT NULL,
		method TEXT NOT NULL DEFAULT 'POST',
		headers TEXT NOT NULL DEFAULT '{}',
		body_tpl TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS notifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_id INTEGER NOT NULL,
		idempotency_key TEXT,
		event_type TEXT,
		url TEXT,
		method TEXT,
		headers TEXT NOT NULL DEFAULT '{}',
		body TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		retry_count INTEGER NOT NULL DEFAULT 0,
		max_retry INTEGER NOT NULL DEFAULT 5,
		retry_interval INTEGER NOT NULL DEFAULT 60,
		next_send_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_error TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_notifications_status_next_send ON notifications(status, next_send_at);
	CREATE INDEX IF NOT EXISTS idx_notifications_provider_id ON notifications(provider_id);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_idempotency_key ON notifications(idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key != '';
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	log.Println("Database schema initialized successfully")
	return nil
}
