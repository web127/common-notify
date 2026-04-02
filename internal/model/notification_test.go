package model

import (
	"testing"
)

func TestNotificationStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   NotificationStatus
		expected string
	}{
		{"StatusPending", StatusPending, "pending"},
		{"StatusProcessing", StatusProcessing, "processing"},
		{"StatusSuccess", StatusSuccess, "success"},
		{"StatusDead", StatusDead, "dead"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("got %s, want %s", tt.status, tt.expected)
			}
		})
	}
}

func TestNotificationStruct(t *testing.T) {
	n := &Notification{
		ID:         123,
		OutBizNo:   "test-001",
		EventType:  "user.registered",
		Status:     StatusPending,
		TargetURL:  "https://example.com",
		Method:     "POST",
		Headers:    "{}",
		Body:       `{"key":"value"}`,
		RetryCount: 0,
		MaxRetries: 5,
	}

	if n.ID != 123 {
		t.Errorf("ID got %d, want 123", n.ID)
	}
	if n.OutBizNo != "test-001" {
		t.Errorf("OutBizNo got %s, want test-001", n.OutBizNo)
	}
	if n.EventType != "user.registered" {
		t.Errorf("EventType got %s, want user.registered", n.EventType)
	}
}

func TestCreateNotificationRequestStruct(t *testing.T) {
	req := &CreateNotificationRequest{
		OutBizNo:  "test-002",
		EventType: "payment.completed",
		TargetURL: "https://example.com/webhook",
		Method:    "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"amount":100}`,
	}

	if req.OutBizNo != "test-002" {
		t.Errorf("OutBizNo got %s, want test-002", req.OutBizNo)
	}
	if req.EventType != "payment.completed" {
		t.Errorf("EventType got %s, want payment.completed", req.EventType)
	}
	if req.TargetURL != "https://example.com/webhook" {
		t.Errorf("TargetURL got %s, want https://example.com/webhook", req.TargetURL)
	}
}

func TestCreateNotificationResponseStruct(t *testing.T) {
	resp := &CreateNotificationResponse{
		ID:      456,
		Status:  StatusSuccess,
		Message: "notification accepted",
	}

	if resp.ID != 456 {
		t.Errorf("ID got %d, want 456", resp.ID)
	}
	if resp.Status != StatusSuccess {
		t.Errorf("Status got %s, want success", resp.Status)
	}
	if resp.Message != "notification accepted" {
		t.Errorf("Message got %s, want notification accepted", resp.Message)
	}
}
