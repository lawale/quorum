package model

import "testing"

func TestRequestStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		status   RequestStatus
		terminal bool
	}{
		{"approved is terminal", StatusApproved, true},
		{"rejected is terminal", StatusRejected, true},
		{"cancelled is terminal", StatusCancelled, true},
		{"expired is terminal", StatusExpired, true},
		{"pending is not terminal", StatusPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.terminal {
				t.Errorf("RequestStatus(%q).IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
			}
		})
	}
}
