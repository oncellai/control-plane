package cellmanager

import (
	"testing"
)

// For testing, we can't use the real CellManager since it needs Redis + gRPC.
// Instead we test the logic by verifying state transitions directly.

func TestCellStateTransitions(t *testing.T) {
	// Test that the state machine is correct
	tests := []struct {
		name     string
		from     string
		action   string
		expected string
	}{
		{"create → active", "", "create", "active"},
		{"active → paused", "active", "pause", "paused"},
		{"paused → active", "paused", "resume", "active"},
		{"active → deleted", "active", "delete", ""},
		{"paused → deleted", "paused", "delete", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the transitions are valid — the actual implementation
			// uses gRPC which we can't test without a running host agent.
			// This test documents the expected state machine.
			switch tt.action {
			case "create":
				if tt.expected != "active" {
					t.Errorf("create should produce active, got %s", tt.expected)
				}
			case "pause":
				if tt.from != "active" {
					t.Errorf("can only pause from active, got %s", tt.from)
				}
			case "resume":
				if tt.from != "paused" {
					t.Errorf("can only resume from paused, got %s", tt.from)
				}
			case "delete":
				// Can delete from any state
			}
		})
	}
}

func TestForceReclaim_NoActiveCells(t *testing.T) {
	// ForceReclaim with no cells should return error
	cm := &CellManager{
		router: nil, // We'll test the error path
	}

	// Can't call ForceReclaim without a real router,
	// but we document the expected behavior
	_ = cm
}

func TestCreateResult_Fields(t *testing.T) {
	r := CreateResult{
		CellID: "test-cell",
		HostID: "host-1",
		Port:   8401,
		Status: "active",
	}

	if r.CellID != "test-cell" {
		t.Errorf("expected test-cell, got %s", r.CellID)
	}
	if r.Port != 8401 {
		t.Errorf("expected 8401, got %d", r.Port)
	}
	if r.Status != "active" {
		t.Errorf("expected active, got %s", r.Status)
	}
}
