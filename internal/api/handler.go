package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/service"
	"github.com/gorilla/mux"
)

// Handler HTTP 处理器
type Handler struct {
	providerService     *service.ProviderService
	notificationService *service.NotificationService
}

// NewHandler 创建 HTTP 处理器
func NewHandler(providerService *service.ProviderService, notificationService *service.NotificationService) *Handler {
	return &Handler{
		providerService:     providerService,
		notificationService: notificationService,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *mux.Router) {
	// 供应商管理
	router.HandleFunc("/api/v1/providers", h.CreateProvider).Methods("POST")
	router.HandleFunc("/api/v1/providers", h.ListProviders).Methods("GET")
	router.HandleFunc("/api/v1/providers/{id}", h.GetProvider).Methods("GET")
	router.HandleFunc("/api/v1/providers/{id}", h.UpdateProvider).Methods("PUT")
	router.HandleFunc("/api/v1/providers/{id}", h.DeleteProvider).Methods("DELETE")

	// 通知管理
	router.HandleFunc("/api/v1/notifications", h.CreateNotification).Methods("POST")
	router.HandleFunc("/api/v1/notifications/{id}", h.GetNotification).Methods("GET")
	router.HandleFunc("/api/v1/notifications/{id}/retry", h.RetryNotification).Methods("POST")
	router.HandleFunc("/api/v1/notifications/status/{status}", h.ListNotificationsByStatus).Methods("GET")

	// 健康检查
	router.HandleFunc("/health", h.HealthCheck).Methods("GET")
}

// 响应辅助函数

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// 供应商处理器

func (h *Handler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var p model.Provider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.providerService.CreateProvider(&p); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, p)
}

func (h *Handler) ListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.providerService.ListProviders()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, providers)
}

func (h *Handler) GetProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid provider id")
		return
	}

	p, err := h.providerService.GetProvider(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		respondError(w, http.StatusNotFound, "provider not found")
		return
	}
	respondJSON(w, http.StatusOK, p)
}

func (h *Handler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid provider id")
		return
	}

	var p model.Provider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.ID = id

	if err := h.providerService.UpdateProvider(&p); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, p)
}

func (h *Handler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid provider id")
		return
	}

	if err := h.providerService.DeleteProvider(id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "provider deleted"})
}

// 通知处理器

func (h *Handler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var n model.Notification
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.notificationService.CreateNotification(&n); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, n)
}

func (h *Handler) GetNotification(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid notification id")
		return
	}

	n, err := h.notificationService.GetNotification(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n == nil {
		respondError(w, http.StatusNotFound, "notification not found")
		return
	}
	respondJSON(w, http.StatusOK, n)
}

func (h *Handler) ListNotificationsByStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	status := model.NotificationStatus(vars["status"])

	// 解析分页参数
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	notifications, err := h.notificationService.ListNotificationsByStatus(status, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, notifications)
}

func (h *Handler) RetryNotification(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid notification id")
		return
	}

	if err := h.notificationService.RetryNotification(id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "notification queued for retry"})
}

// 健康检查

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
