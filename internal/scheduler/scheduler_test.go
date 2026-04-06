package scheduler

import (
	"context"
	"testing"
)

func TestPickHost_NoHosts(t *testing.T) {
	s := &Scheduler{hosts: []Host{}}
	_, err := s.PickHost(context.Background(), "any-cell")
	if err != ErrNoHosts {
		t.Fatalf("expected ErrNoHosts, got %v", err)
	}
}

func TestPickHost_SingleHost_NilRouter(t *testing.T) {
	s := &Scheduler{
		router: nil,
		hosts:  []Host{{ID: "host-1", Address: "10.0.20.5", GRPCPort: 50051}},
	}
	host, err := s.PickHost(context.Background(), "any-cell")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if host.ID != "host-1" {
		t.Fatalf("expected host-1, got %s", host.ID)
	}
}

func TestPickHost_MultipleHosts_NilRouter(t *testing.T) {
	s := &Scheduler{
		router: nil,
		hosts: []Host{
			{ID: "host-1", Address: "10.0.20.5", GRPCPort: 50051},
			{ID: "host-2", Address: "10.0.20.6", GRPCPort: 50051},
		},
	}
	host, err := s.PickHost(context.Background(), "any-cell")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if host.ID != "host-1" {
		t.Fatalf("expected host-1 (first), got %s", host.ID)
	}
}

func TestErrNoHosts_Message(t *testing.T) {
	if ErrNoHosts.Error() != "no hosts available" {
		t.Errorf("unexpected: %s", ErrNoHosts.Error())
	}
}
