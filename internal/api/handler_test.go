package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/repository"
	"code.byted.org/fintech_cf/common-notify/internal/service"
	"github.com/gorilla/mux"
)

func setupTestHandler(t *testing.T) (*Handler, *repository.DB, func()) {
	t.Helper()

	db, err := repository.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}

	providerRepo := repository.NewProviderRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)

	providerService := service.NewProviderService(providerRepo)
	notificationService := service.NewNotificationService(notificationRepo, providerRepo)

	handler := NewHandler(providerService, notificationService)

	cleanup := func() {
		db.Close()
	}

	return handler, db, cleanup
}

func TestHandler_HealthCheck(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	err := json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("Expected status ok, got %s", result["status"])
	}
}

func TestHandler_CreateProvider(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "valid provider",
			body: map[string]interface{}{
				"name":     "test-provider",
				"base_url": "https://example.com",
				"method":   "POST",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid body",
			body:       "invalid json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing required fields",
			body: map[string]interface{}{
				"name": "only-name",
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyBytes []byte
			var err error
			if str, ok := tt.body.(string); ok {
				bodyBytes = []byte(str)
			} else {
				bodyBytes, err = json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("Failed to marshal body: %v", err)
				}
			}

			req := httptest.NewRequest("POST", "/api/v1/providers", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateProvider(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestHandler_ProviderCRUD(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create provider
	providerBody := map[string]interface{}{
		"name":     "crud-provider",
		"base_url": "https://crud.com",
		"method":   "POST",
	}
	bodyBytes, _ := json.Marshal(providerBody)

	req := httptest.NewRequest("POST", "/api/v1/providers", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.CreateProvider(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Create failed: %d", w.Code)
	}

	var createdProvider model.Provider
	json.NewDecoder(w.Body).Decode(&createdProvider)
	if createdProvider.ID == 0 {
		t.Fatal("Provider ID not set")
	}

	// Get provider
	req = httptest.NewRequest("GET", "/api/v1/providers/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w = httptest.NewRecorder()
	handler.GetProvider(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Get failed: %d", w.Code)
	}

	// List providers
	req = httptest.NewRequest("GET", "/api/v1/providers", nil)
	w = httptest.NewRecorder()
	handler.ListProviders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("List failed: %d", w.Code)
	}

	// Update provider
	updateBody := map[string]interface{}{
		"name":     "updated-provider",
		"base_url": "https://updated.com",
		"method":   "PUT",
	}
	bodyBytes, _ = json.Marshal(updateBody)
	req = httptest.NewRequest("PUT", "/api/v1/providers/1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w = httptest.NewRecorder()
	handler.UpdateProvider(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Update failed: %d", w.Code)
	}

	// Delete provider
	req = httptest.NewRequest("DELETE", "/api/v1/providers/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w = httptest.NewRecorder()
	handler.DeleteProvider(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Delete failed: %d", w.Code)
	}
}

func TestHandler_CreateNotification(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a provider first
	providerRepo := repository.NewProviderRepository(db)
	provider := &model.Provider{
		Name:    "notif-provider",
		BaseURL: "https://example.com",
		Method:  "POST",
	}
	err := providerRepo.Create(provider)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "valid notification",
			body: map[string]interface{}{
				"provider_id": provider.ID,
				"body":        `{"message": "test"}`,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid body",
			body:       "invalid json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid provider",
			body: map[string]interface{}{
				"provider_id": 999,
				"body":        `{"message": "test"}`,
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyBytes []byte
			var err error
			if str, ok := tt.body.(string); ok {
				bodyBytes = []byte(str)
			} else {
				bodyBytes, err = json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("Failed to marshal body: %v", err)
				}
			}

			req := httptest.NewRequest("POST", "/api/v1/notifications", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateNotification(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestHandler_GetNotification(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create provider and notification
	providerRepo := repository.NewProviderRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)

	provider := &model.Provider{
		Name:    "get-notif-provider",
		BaseURL: "https://example.com",
	}
	err := providerRepo.Create(provider)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	notification := &model.Notification{
		ProviderID: provider.ID,
		Body:       `{"test": "data"}`,
	}
	err = notificationRepo.Create(notification)
	if err != nil {
		t.Fatalf("Failed to create notification: %v", err)
	}

	// Test valid ID
	req := httptest.NewRequest("GET", "/api/v1/notifications/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()
	handler.GetNotification(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test invalid ID format
	req = httptest.NewRequest("GET", "/api/v1/notifications/invalid", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
	w = httptest.NewRecorder()
	handler.GetNotification(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Test non-existent ID
	req = httptest.NewRequest("GET", "/api/v1/notifications/999", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	w = httptest.NewRecorder()
	handler.GetNotification(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_RetryNotification(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create provider and notification
	providerRepo := repository.NewProviderRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)

	provider := &model.Provider{
		Name:    "retry-provider",
		BaseURL: "https://example.com",
	}
	err := providerRepo.Create(provider)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	notification := &model.Notification{
		ProviderID: provider.ID,
		Body:       `{"test": "data"}`,
		Status:     model.StatusFailed,
	}
	err = notificationRepo.Create(notification)
	if err != nil {
		t.Fatalf("Failed to create notification: %v", err)
	}

	// Test retry
	req := httptest.NewRequest("POST", "/api/v1/notifications/1/retry", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()
	handler.RetryNotification(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandler_ListNotificationsByStatus(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create provider and notifications
	providerRepo := repository.NewProviderRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)

	provider := &model.Provider{
		Name:    "list-provider",
		BaseURL: "https://example.com",
	}
	err := providerRepo.Create(provider)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create notifications with different statuses
	statuses := []model.NotificationStatus{
		model.StatusPending,
		model.StatusSuccess,
		model.StatusFailed,
	}

	for _, status := range statuses {
		notification := &model.Notification{
			ProviderID: provider.ID,
			Body:       `{"test": "data"}`,
			Status:     status,
		}
		err = notificationRepo.Create(notification)
		if err != nil {
			t.Fatalf("Failed to create notification: %v", err)
		}
	}

	// Test list by status
	req := httptest.NewRequest("GET", "/api/v1/notifications/status/pending?limit=10&offset=0", nil)
	req = mux.SetURLVars(req, map[string]string{"status": "pending"})
	w := httptest.NewRecorder()
	handler.ListNotificationsByStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
