// Package api 提供 HTTP API 处理层
// 接收上游业务系统的通知请求，实现幂等去重
package api

import (
	"encoding/json"
	"net/http"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/store"
	"code.byted.org/fintech_cf/common-notify/internal/worker"
)

// Handler HTTP 请求处理器
// 封装存储层和 worker 层的交互
type Handler struct {
	store  *store.SQLiteStore // 存储层
	worker *worker.Worker     // worker 层
}

// NewHandler 创建一个新的 Handler 实例
// store: 存储层实例
// worker: worker 层实例
func NewHandler(store *store.SQLiteStore, worker *worker.Worker) *Handler {
	return &Handler{store: store, worker: worker}
}

// CreateNotification 处理创建通知请求
// 接口：POST /api/v1/notifications
// 功能：
// 1. 验证 HTTP 方法
// 2. 解析请求体
// 3. 幂等检查（通过 out_biz_no）
// 4. 创建通知任务
// 5. 提交到 worker 队列
// 6. 返回响应
func (h *Handler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	// 只允许 POST 方法
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求体
	var req model.CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// 必填字段校验
	if req.EventType == "" || req.TargetURL == "" {
		http.Error(w, "event_type and target_url are required", http.StatusBadRequest)
		return
	}

	// 幂等检查：如果 out_biz_no 已存在，直接返回已有记录
	if req.OutBizNo != "" {
		if existing, _ := h.store.GetNotificationByOutBizNo(req.OutBizNo); existing != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(model.CreateNotificationResponse{
				ID:      existing.ID,
				Status:  existing.Status,
				Message: "notification already exists (idempotent)",
			})
			return
		}
	}

	// 创建通知任务并持久化
	n, err := h.store.CreateNotification(&req)
	if err != nil {
		http.Error(w, "failed to create notification", http.StatusInternalServerError)
		return
	}

	// 提交到 worker 队列进行投递
	h.worker.Submit(n)

	// 返回 202 Accepted，表示任务已接收
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(model.CreateNotificationResponse{
		ID:      n.ID,
		Status:  n.Status,
		Message: "notification accepted",
	})
}

// RegisterRoutes 注册 HTTP 路由
// mux: HTTP 复用器
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/notifications", h.CreateNotification)
}
