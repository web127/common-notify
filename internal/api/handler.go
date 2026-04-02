package api

import (
	"encoding/json"
	"net/http"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/store"
	"code.byted.org/fintech_cf/common-notify/internal/worker"
)

type Handler struct {
	store  *store.SQLiteStore
	worker *worker.Worker
}

func NewHandler(store *store.SQLiteStore, worker *worker.Worker) *Handler {
	return &Handler{store: store, worker: worker}
}

func (h *Handler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req model.CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.EventType == "" || req.TargetURL == "" {
		http.Error(w, "event_type and target_url are required", http.StatusBadRequest)
		return
	}

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

	n, err := h.store.CreateNotification(&req)
	if err != nil {
		http.Error(w, "failed to create notification", http.StatusInternalServerError)
		return
	}

	h.worker.Submit(n)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(model.CreateNotificationResponse{
		ID:      n.ID,
		Status:  n.Status,
		Message: "notification accepted",
	})
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/notifications", h.CreateNotification)
}
