package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Notification 通知记录
type Notification struct {
	ID            int64        `json:"id" db:"id"`
	OutBizNo      string       `json:"out_biz_no" db:"out_biz_no"`
	VendorID      string       `json:"vendor_id" db:"vendor_id"`
	EventType     string       `json:"event_type" db:"event_type"`
	Payload       JSONMap      `json:"payload" db:"payload"`
	Status        string       `json:"status" db:"status"`
	Attempts      int          `json:"attempts" db:"attempts"`
	LastAttemptAt *time.Time   `json:"last_attempt_at" db:"last_attempt_at"`
	NextAttemptAt time.Time    `json:"next_attempt_at" db:"next_attempt_at"`
	ErrorMessage  string       `json:"error_message" db:"error_message"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
}

// Vendor 供应商配置
type Vendor struct {
	ID            string       `json:"id" db:"id"`
	Name          string       `json:"name" db:"name"`
	EndpointURL   string       `json:"endpoint_url" db:"endpoint_url"`
	Method        string       `json:"method" db:"method"`
	Headers       JSONMap      `json:"headers" db:"headers"`
	BodyTemplate  string       `json:"body_template" db:"body_template"`
	TimeoutMs     int          `json:"timeout_ms" db:"timeout_ms"`
	SkipTLSVerify bool         `json:"skip_tls_verify" db:"skip_tls_verify"`
	CACert        string       `json:"ca_cert" db:"ca_cert"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
}

// JSONMap 自定义 JSON 类型
type JSONMap map[string]interface{}

// Value 实现 driver.Valuer 接口
func (j JSONMap) Value() (driver.Value, error) {
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
func (j *JSONMap) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

// 状态常量
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusSuccess    = "success"
	StatusFailed     = "failed"
)

// 通知状态是否终态
func (n *Notification) IsTerminal() bool {
	return n.Status == StatusSuccess || n.Status == StatusFailed
}
