package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/model"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	t.Helper()

	// 创建临时数据库文件
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}

	return db, cleanup
}

func TestProviderRepository_CRUD(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewProviderRepository(db)

	// Test Create
	provider := &model.Provider{
		Name:    "test-provider",
		BaseURL: "https://example.com",
		Method:  "POST",
		Headers: model.StringMap{"Content-Type": "application/json"},
		BodyTpl: `{"message": "{{text}}"}`,
	}

	err := repo.Create(provider)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	if provider.ID == 0 {
		t.Error("Expected provider ID to be set, got 0")
	}

	// Test GetByID
	fetched, err := repo.GetByID(provider.ID)
	if err != nil {
		t.Fatalf("Failed to get provider by ID: %v", err)
	}
	if fetched == nil {
		t.Fatal("Expected provider, got nil")
	}
	if fetched.Name != provider.Name {
		t.Errorf("Name mismatch: got %s, want %s", fetched.Name, provider.Name)
	}

	// Test GetByName
	fetchedByName, err := repo.GetByName(provider.Name)
	if err != nil {
		t.Fatalf("Failed to get provider by name: %v", err)
	}
	if fetchedByName == nil {
		t.Fatal("Expected provider by name, got nil")
	}
	if fetchedByName.ID != provider.ID {
		t.Errorf("ID mismatch: got %d, want %d", fetchedByName.ID, provider.ID)
	}

	// Test List
	providers, err := repo.List()
	if err != nil {
		t.Fatalf("Failed to list providers: %v", err)
	}
	if len(providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(providers))
	}

	// Test Update
	provider.Name = "updated-provider"
	err = repo.Update(provider)
	if err != nil {
		t.Fatalf("Failed to update provider: %v", err)
	}
	updated, err := repo.GetByID(provider.ID)
	if err != nil {
		t.Fatalf("Failed to get updated provider: %v", err)
	}
	if updated.Name != "updated-provider" {
		t.Errorf("Expected name updated-provider, got %s", updated.Name)
	}

	// Test Delete
	err = repo.Delete(provider.ID)
	if err != nil {
		t.Fatalf("Failed to delete provider: %v", err)
	}
	deleted, err := repo.GetByID(provider.ID)
	if err != nil {
		t.Fatalf("Failed to check deleted provider: %v", err)
	}
	if deleted != nil {
		t.Error("Expected nil after deletion, got provider")
	}
}

func TestProviderRepository_GetNonExistent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewProviderRepository(db)

	// Get non-existent ID
	fetched, err := repo.GetByID(999)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fetched != nil {
		t.Error("Expected nil for non-existent ID")
	}

	// Get non-existent name
	fetchedByName, err := repo.GetByName("non-existent")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fetchedByName != nil {
		t.Error("Expected nil for non-existent name")
	}
}

func TestNotificationRepository_CRUD(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	providerRepo := NewProviderRepository(db)
	notificationRepo := NewNotificationRepository(db)

	// Create a provider first
	provider := &model.Provider{
		Name:    "test-provider",
		BaseURL: "https://example.com",
		Method:  "POST",
	}
	err := providerRepo.Create(provider)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Test Create Notification
	notification := &model.Notification{
		ProviderID:    provider.ID,
		URL:           "https://example.com/notify",
		Method:        "POST",
		Headers:       model.StringMap{"X-API-Key": "secret"},
		Body:          `{"text": "hello"}`,
		Status:        model.StatusPending,
		MaxRetry:      3,
		RetryInterval: 30,
	}

	err = notificationRepo.Create(notification)
	if err != nil {
		t.Fatalf("Failed to create notification: %v", err)
	}
	if notification.ID == 0 {
		t.Error("Expected notification ID to be set, got 0")
	}

	// Test GetByID
	fetched, err := notificationRepo.GetByID(notification.ID)
	if err != nil {
		t.Fatalf("Failed to get notification: %v", err)
	}
	if fetched == nil {
		t.Fatal("Expected notification, got nil")
	}
	if fetched.Body != notification.Body {
		t.Errorf("Body mismatch: got %s, want %s", fetched.Body, notification.Body)
	}

	// Test Update
	notification.Status = model.StatusSending
	notification.LastError = "test error"
	err = notificationRepo.Update(notification)
	if err != nil {
		t.Fatalf("Failed to update notification: %v", err)
	}

	// Test UpdateStatus
	err = notificationRepo.UpdateStatus(notification.ID, model.StatusFailed, "final error")
	if err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	// Test MarkForRetry
	nextSendAt := time.Now().Add(1 * time.Minute)
	err = notificationRepo.MarkForRetry(notification.ID, 1, nextSendAt, "retry error")
	if err != nil {
		t.Fatalf("Failed to mark for retry: %v", err)
	}

	// Test MarkFailed
	err = notificationRepo.MarkFailed(notification.ID, "max retries reached")
	if err != nil {
		t.Fatalf("Failed to mark as failed: %v", err)
	}
}

func TestNotificationRepository_List(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	providerRepo := NewProviderRepository(db)
	notificationRepo := NewNotificationRepository(db)

	// Create provider
	provider := &model.Provider{
		Name:    "test-provider",
		BaseURL: "https://example.com",
		Method:  "POST",
	}
	err := providerRepo.Create(provider)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create multiple notifications
	now := time.Now()
	notifications := []*model.Notification{
		{
			ProviderID: provider.ID,
			Status:     model.StatusPending,
			NextSendAt: now.Add(-1 * time.Minute),
		},
		{
			ProviderID: provider.ID,
			Status:     model.StatusFailed,
			NextSendAt: now.Add(-2 * time.Minute),
		},
		{
			ProviderID: provider.ID,
			Status:     model.StatusSuccess,
			NextSendAt: now,
		},
	}

	for _, n := range notifications {
		err = notificationRepo.Create(n)
		if err != nil {
			t.Fatalf("Failed to create notification: %v", err)
		}
	}

	// Test ListPending
	pending, err := notificationRepo.ListPending(10)
	if err != nil {
		t.Fatalf("Failed to list pending: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending notifications, got %d", len(pending))
	}

	// Test ListByStatus
	byStatus, err := notificationRepo.ListByStatus(model.StatusSuccess, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list by status: %v", err)
	}
	if len(byStatus) != 1 {
		t.Errorf("Expected 1 success notification, got %d", len(byStatus))
	}
}
