package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/store"
	"code.byted.org/fintech_cf/common-notify/internal/worker"
)

func setupTestAPI(t *testing.T) (*Handler, *store.SQLiteStore, func()) {
	dbPath := "test_api_notifications.db"
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	w := worker.NewWorker(s, 1, 100)
	h := NewHandler(s, w)

	cleanup := func() {
		s.Close()
		os.Remove(dbPath)
	}

	return h, s, cleanup
}

func TestCreateNotification_Success(t *testing.T) {
	h, _, cleanup := setupTestAPI(t)
	defer cleanup()

	reqBody := map[string]interface{}{
		"out_biz_no":  "test-api-001",
		"event_type":  "user.registered",
		"target_url":  "https://example.com/webhook",
		"method":      "POST",
		"headers":     map[string]string{"Content-Type": "application/json"},
		"body":        `{"user_id": 123}`,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateNotification(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("StatusCode got %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	var respBody model.CreateNotificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if respBody.Status != model.StatusPending {
		t.Errorf("Status got %s, want pending", respBody.Status)
	}
	if respBody.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestCreateNotification_Idempotent(t *testing.T) {
	h, _, cleanup := setupTestAPI(t)
	defer cleanup()

	reqBody := map[string]interface{}{
		"out_biz_no":  "test-api-idempotent-001",
		"event_type":  "user.registered",
		"target_url":  "https://example.com/webhook",
	}
	body, _ := json.Marshal(reqBody)

	req1 := httptest.NewRequest("POST", "/api/v1/notifications", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	h.CreateNotification(w1, req1)

	var resp1 model.CreateNotificationResponse
	json.NewDecoder(w1.Result().Body).Decode(&resp1)

	req2 := httptest.NewRequest("POST", "/api/v1/notifications", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.CreateNotification(w2, req2)

	resp2 := w2.Result()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("StatusCode got %d, want %d", resp2.StatusCode, http.StatusOK)
	}

	var respBody2 model.CreateNotificationResponse
	json.NewDecoder(resp2.Body).Decode(&respBody2)

	if respBody2.ID != resp1.ID {
		t.Errorf("ID mismatch: got %d, want %d", respBody2.ID, resp1.ID)
	}
}

func TestCreateNotification_MissingRequiredFields(t *testing.T) {
	h, _, cleanup := setupTestAPI(t)
	defer cleanup()

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{
			"missing event_type",
			map[string]interface{}{
				"out_biz_no": "test-missing-001",
				"target_url": "https://example.com",
			},
		},
		{
			"missing target_url",
			map[string]interface{}{
				"out_biz_no": "test-missing-002",
				"event_type": "user.registered",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/v1/notifications", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.CreateNotification(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("StatusCode got %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

func TestCreateNotification_MethodNotAllowed(t *testing.T) {
	h, _, cleanup := setupTestAPI(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/notifications", nil)
	w := httptest.NewRecorder()

	h.CreateNotification(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("StatusCode got %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestRegisterRoutes(t *testing.T) {
	h, _, cleanup := setupTestAPI(t)
	defer cleanup()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/api/v1/notifications", bytes.NewBufferString(`{"event_type":"test","target_url":"https://example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("StatusCode got %d, want %d", w.Code, http.StatusAccepted)
	}
}
