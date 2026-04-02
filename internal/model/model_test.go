package model

import (
	"testing"
)

func TestStringMap_Value(t *testing.T) {
	tests := []struct {
		name    string
		m       StringMap
		wantErr bool
	}{
		{
			name:    "nil map",
			m:       nil,
			wantErr: false,
		},
		{
			name:    "empty map",
			m:       StringMap{},
			wantErr: false,
		},
		{
			name:    "single key-value",
			m:       StringMap{"Content-Type": "application/json"},
			wantErr: false,
		},
		{
			name:    "multiple key-values",
			m:       StringMap{"Content-Type": "application/json", "Authorization": "Bearer token"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.m.Value()
			if (err != nil) != tt.wantErr {
				t.Errorf("StringMap.Value() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Just check we got a non-nil value without error
			if got == nil {
				t.Error("StringMap.Value() got nil, want non-nil")
			}
		})
	}
}

func TestStringMap_Scan(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    StringMap
		wantErr bool
	}{
		{
			name:    "nil value",
			value:   nil,
			want:    StringMap{},
			wantErr: false,
		},
		{
			name:    "empty JSON",
			value:   []byte("{}"),
			want:    StringMap{},
			wantErr: false,
		},
		{
			name:    "single key-value",
			value:   []byte(`{"Content-Type":"application/json"}`),
			want:    StringMap{"Content-Type": "application/json"},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			value:   []byte(`invalid`),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "wrong type",
			value:   "not bytes",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := StringMap{}
			err := m.Scan(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("StringMap.Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if tt.want != nil && len(m) != len(tt.want) {
					t.Errorf("StringMap.Scan() got len = %v, want len = %v", len(m), len(tt.want))
				}
				for k, v := range tt.want {
					if m[k] != v {
						t.Errorf("StringMap.Scan() got[%s] = %v, want %v", k, m[k], v)
					}
				}
			}
		})
	}
}

func TestNotificationStatus_Constants(t *testing.T) {
	if StatusPending != "pending" {
		t.Errorf("StatusPending = %v, want pending", StatusPending)
	}
	if StatusSending != "sending" {
		t.Errorf("StatusSending = %v, want sending", StatusSending)
	}
	if StatusSuccess != "success" {
		t.Errorf("StatusSuccess = %v, want success", StatusSuccess)
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %v, want failed", StatusFailed)
	}
}
