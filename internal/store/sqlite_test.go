package store

import (
	"os"
	"testing"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/model"
)

func setupTestDB(t *testing.T) (*SQLiteStore, func()) {
	dbPath := "test_notifications.db"
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Remove(dbPath)
	}

	return store, cleanup
}

func TestCreateNotification(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	req := &model.CreateNotificationRequest{
		OutBizNo:  "test-create-001",
		EventType: "user.registered",
		TargetURL: "https://example.com/webhook",
		Method:    "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"user_id": 123}`,
	}

	n, err := store.CreateNotification(req)
	if err != nil {
		t.Fatalf("CreateNotification failed: %v", err)
	}

	if n.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if n.OutBizNo != "test-create-001" {
		t.Errorf("OutBizNo got %s, want test-create-001", n.OutBizNo)
	}
	if n.EventType != "user.registered" {
		t.Errorf("EventType got %s, want user.registered", n.EventType)
	}
	if n.Status != model.StatusPending {
		t.Errorf("Status got %s, want pending", n.Status)
	}
	if n.Method != "POST" {
		t.Errorf("Method got %s, want POST", n.Method)
	}
}

func TestGetNotificationByOutBizNo(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	req := &model.CreateNotificationRequest{
		OutBizNo:  "test-get-001",
		EventType: "test.event",
		TargetURL: "https://example.com",
	}

	created, err := store.CreateNotification(req)
	if err != nil {
		t.Fatalf("CreateNotification failed: %v", err)
	}

	fetched, err := store.GetNotificationByOutBizNo("test-get-001")
	if err != nil {
		t.Fatalf("GetNotificationByOutBizNo failed: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("ID mismatch: got %d, want %d", fetched.ID, created.ID)
	}
}

func TestGetNotificationByID(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	req := &model.CreateNotificationRequest{
		OutBizNo:  "test-get-id-001",
		EventType: "test.event",
		TargetURL: "https://example.com",
	}

	created, err := store.CreateNotification(req)
	if err != nil {
		t.Fatalf("CreateNotification failed: %v", err)
	}

	fetched, err := store.GetNotificationByID(created.ID)
	if err != nil {
		t.Fatalf("GetNotificationByID failed: %v", err)
	}

	if fetched.OutBizNo != "test-get-id-001" {
		t.Errorf("OutBizNo mismatch")
	}
}

func TestMarkSuccess(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	req := &model.CreateNotificationRequest{
		OutBizNo:  "test-success-001",
		EventType: "test.event",
		TargetURL: "https://example.com",
	}

	created, err := store.CreateNotification(req)
	if err != nil {
		t.Fatalf("CreateNotification failed: %v", err)
	}

	err = store.MarkSuccess(created.ID)
	if err != nil {
		t.Fatalf("MarkSuccess failed: %v", err)
	}

	updated, err := store.GetNotificationByID(created.ID)
	if err != nil {
		t.Fatalf("GetNotificationByID failed: %v", err)
	}

	if updated.Status != model.StatusSuccess {
		t.Errorf("Status got %s, want success", updated.Status)
	}
}

func TestMarkDead(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	req := &model.CreateNotificationRequest{
		OutBizNo:  "test-dead-001",
		EventType: "test.event",
		TargetURL: "https://example.com",
	}

	created, err := store.CreateNotification(req)
	if err != nil {
		t.Fatalf("CreateNotification failed: %v", err)
	}

	err = store.MarkDead(created.ID, "connection timeout")
	if err != nil {
		t.Fatalf("MarkDead failed: %v", err)
	}

	updated, err := store.GetNotificationByID(created.ID)
	if err != nil {
		t.Fatalf("GetNotificationByID failed: %v", err)
	}

	if updated.Status != model.StatusDead {
		t.Errorf("Status got %s, want dead", updated.Status)
	}
	if updated.LastError != "connection timeout" {
		t.Errorf("LastError got %s, want connection timeout", updated.LastError)
	}
}

func TestScheduleRetry(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	req := &model.CreateNotificationRequest{
		OutBizNo:  "test-retry-001",
		EventType: "test.event",
		TargetURL: "https://example.com",
	}

	created, err := store.CreateNotification(req)
	if err != nil {
		t.Fatalf("CreateNotification failed: %v", err)
	}

	nextRetryAt := time.Now().Add(1 * time.Hour)
	err = store.ScheduleRetry(created.ID, 1, nextRetryAt, "temporary error")
	if err != nil {
		t.Fatalf("ScheduleRetry failed: %v", err)
	}

	updated, err := store.GetNotificationByID(created.ID)
	if err != nil {
		t.Fatalf("GetNotificationByID failed: %v", err)
	}

	if updated.Status != model.StatusPending {
		t.Errorf("Status got %s, want pending", updated.Status)
	}
	if updated.RetryCount != 1 {
		t.Errorf("RetryCount got %d, want 1", updated.RetryCount)
	}
}

func TestListPendingNotifications(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	req1 := &model.CreateNotificationRequest{
		OutBizNo:  "test-list-001",
		EventType: "test.event",
		TargetURL: "https://example.com",
	}
	req2 := &model.CreateNotificationRequest{
		OutBizNo:  "test-list-002",
		EventType: "test.event",
		TargetURL: "https://example.com",
	}

	_, err := store.CreateNotification(req1)
	if err != nil {
		t.Fatalf("CreateNotification 1 failed: %v", err)
	}
	_, err = store.CreateNotification(req2)
	if err != nil {
		t.Fatalf("CreateNotification 2 failed: %v", err)
	}

	list, err := store.ListPendingNotifications(10)
	if err != nil {
		t.Fatalf("ListPendingNotifications failed: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("List length got %d, want 2", len(list))
	}
}
