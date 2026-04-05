package scheduler

import (
	"context"
	"log/slog"

	"github.com/oncellai/control-plane/internal/router"
)

type Host struct {
	ID       string
	Address  string
	GRPCPort int
}

type Scheduler struct {
	router *router.Router
	hosts  []Host
}

func New(r *router.Router) *Scheduler {
	// MVP: single host, configured statically
	// Later: discover hosts from Redis heartbeats
	return &Scheduler{
		router: r,
		hosts: []Host{
			{ID: "host-1", Address: "localhost", GRPCPort: 50051},
		},
	}
}

// PickHost selects the best host for a new cell.
// Prefers hosts with existing NVMe cache for the customer.
func (s *Scheduler) PickHost(ctx context.Context, cellID string) (*Host, error) {
	if len(s.hosts) == 0 {
		return nil, ErrNoHosts
	}

	// Check if any host has cached data for this cell
	for _, host := range s.hosts {
		metrics, err := s.router.GetHostMetrics(ctx, host.ID)
		if err != nil || metrics == nil {
			continue
		}
		for _, cached := range metrics.CachedCustomers {
			if cached == cellID {
				slog.Info("scheduler: cache hit", "cell_id", cellID, "host", host.ID)
				return &host, nil
			}
		}
	}

	// No cache hit — pick least loaded host
	var best *Host
	var bestCPU int = 1<<31 - 1

	for i := range s.hosts {
		metrics, err := s.router.GetHostMetrics(ctx, s.hosts[i].ID)
		if err != nil || metrics == nil {
			// No metrics = assume empty host
			return &s.hosts[i], nil
		}
		if metrics.CPUUsed < bestCPU {
			bestCPU = metrics.CPUUsed
			best = &s.hosts[i]
		}
	}

	if best != nil {
		slog.Info("scheduler: least loaded", "host", best.ID, "cpu_used", bestCPU)
		return best, nil
	}

	return &s.hosts[0], nil
}

type SchedulerError string

func (e SchedulerError) Error() string { return string(e) }

const ErrNoHosts = SchedulerError("no hosts available")
