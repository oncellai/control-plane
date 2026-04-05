package cellmanager

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/oncellai/control-plane/internal/hostclient"
	pb "github.com/oncellai/control-plane/internal/hostclient/pb"
	"github.com/oncellai/control-plane/internal/router"
	"github.com/oncellai/control-plane/internal/scheduler"
)

type CellManager struct {
	router    *router.Router
	scheduler *scheduler.Scheduler
	hosts     *hostclient.Pool
}

func New(r *router.Router, s *scheduler.Scheduler) *CellManager {
	return &CellManager{
		router:    r,
		scheduler: s,
		hosts:     hostclient.NewPool(),
	}
}

type CreateResult struct {
	CellID string `json:"cell_id"`
	HostID string `json:"host_id"`
	Port   int    `json:"port"`
	Status string `json:"status"`
}

func (cm *CellManager) Create(ctx context.Context, cellID, customerID, developerID string) (*CreateResult, error) {
	// Pick a host
	host, err := cm.scheduler.PickHost(ctx, cellID)
	if err != nil {
		// Try force reclaim if no capacity
		if _, reclaimErr := cm.ForceReclaim(ctx); reclaimErr != nil {
			return nil, fmt.Errorf("scheduler: %w (reclaim also failed: %v)", err, reclaimErr)
		}
		// Retry after reclaim
		host, err = cm.scheduler.PickHost(ctx, cellID)
		if err != nil {
			return nil, fmt.Errorf("scheduler: %w", err)
		}
	}

	// Connect to Host Agent
	client, err := cm.hosts.Get(host.ID, host.Address, host.GRPCPort)
	if err != nil {
		return nil, fmt.Errorf("connect to host %s: %w", host.ID, err)
	}

	// Create cell via gRPC
	resp, err := client.CreateCell(ctx, &pb.CreateCellRequest{
		CellId:      cellID,
		CustomerId:  customerID,
		DeveloperId: developerID,
		Spec: &pb.CellSpec{
			CpuMillicores: 4000,
			MemoryMb:      8192,
			StorageGb:     50,
		},
		AgentImage: "default",
	})
	if err != nil {
		return nil, fmt.Errorf("create cell: %w", err)
	}

	// Register route
	route := router.CellRoute{
		Host:   host.Address,
		Port:   int(resp.Port),
		Status: "active",
		CellID: cellID,
	}
	if err := cm.router.SetRoute(ctx, cellID, route); err != nil {
		return nil, fmt.Errorf("set route: %w", err)
	}
	cm.router.SetLastActive(ctx, cellID)

	slog.Info("cell created", "cell_id", cellID, "host", host.ID, "port", resp.Port)

	return &CreateResult{
		CellID: cellID,
		HostID: host.ID,
		Port:   int(resp.Port),
		Status: "active",
	}, nil
}

func (cm *CellManager) Pause(ctx context.Context, cellID string) error {
	route, err := cm.router.GetRoute(ctx, cellID)
	if err != nil || route == nil {
		return fmt.Errorf("cell not found: %s", cellID)
	}

	// Find host client from route
	client, err := cm.hostForRoute(route)
	if err != nil {
		return fmt.Errorf("connect to host: %w", err)
	}

	// Pause via gRPC (Host Agent snapshots to S3, then stops sandbox)
	if _, err := client.PauseCell(ctx, cellID); err != nil {
		return fmt.Errorf("pause cell: %w", err)
	}

	route.Status = "paused"
	cm.router.SetRoute(ctx, cellID, *route)

	slog.Info("cell paused", "cell_id", cellID)
	return nil
}

func (cm *CellManager) Resume(ctx context.Context, cellID string) (*CreateResult, error) {
	route, err := cm.router.GetRoute(ctx, cellID)
	if err != nil || route == nil {
		return nil, fmt.Errorf("cell not found: %s", cellID)
	}

	// Find host client from route
	client, err := cm.hostForRoute(route)
	if err != nil {
		return nil, fmt.Errorf("connect to host: %w", err)
	}

	// Resume via gRPC (Host Agent checks NVMe cache or restores from S3)
	resp, err := client.ResumeCell(ctx, cellID)
	if err != nil {
		return nil, fmt.Errorf("resume cell: %w", err)
	}

	route.Status = "active"
	route.Port = int(resp.Port)
	cm.router.SetRoute(ctx, cellID, *route)
	cm.router.SetLastActive(ctx, cellID)

	slog.Info("cell resumed", "cell_id", cellID, "port", resp.Port)

	return &CreateResult{
		CellID: cellID,
		HostID: "host-1",
		Port:   int(resp.Port),
		Status: "active",
	}, nil
}

func (cm *CellManager) Delete(ctx context.Context, cellID string) error {
	route, err := cm.router.GetRoute(ctx, cellID)
	if err != nil || route == nil {
		// Already gone — just clean up routing
		cm.router.DeleteRoute(ctx, cellID)
		return nil
	}

	// Delete via gRPC (Host Agent wipes NVMe, removes sandbox)
	client, err := cm.hostForRoute(route)
	if err == nil {
		if err := client.DeleteCell(ctx, cellID); err != nil {
			slog.Warn("delete cell on host failed", "cell_id", cellID, "err", err)
		}
	}

	cm.router.DeleteRoute(ctx, cellID)

	slog.Info("cell deleted", "cell_id", cellID)
	return nil
}

func (cm *CellManager) GetRoute(ctx context.Context, cellID string) (*router.CellRoute, error) {
	return cm.router.GetRoute(ctx, cellID)
}

// ForceReclaim pauses the least recently active cell to free capacity.
func (cm *CellManager) ForceReclaim(ctx context.Context) (string, error) {
	cells, err := cm.router.GetActiveCells(ctx)
	if err != nil {
		return "", fmt.Errorf("list active cells: %w", err)
	}

	if len(cells) == 0 {
		return "", fmt.Errorf("no active cells to reclaim")
	}

	var oldestID string
	var oldestTime int64 = 1<<62 - 1

	for _, cellID := range cells {
		lastActive, err := cm.router.GetLastActive(ctx, cellID)
		if err != nil {
			continue
		}
		if lastActive < oldestTime {
			oldestTime = lastActive
			oldestID = cellID
		}
	}

	if oldestID == "" {
		return "", fmt.Errorf("could not find reclaimable cell")
	}

	now := time.Now().Unix()
	idleSecs := now - oldestTime
	if idleSecs < 300 {
		return "", fmt.Errorf("no cells idle long enough to reclaim (oldest idle: %ds)", idleSecs)
	}

	slog.Info("force reclaim: pausing least active cell", "cell_id", oldestID, "idle_secs", idleSecs)

	if err := cm.Pause(ctx, oldestID); err != nil {
		return "", fmt.Errorf("failed to pause %s: %w", oldestID, err)
	}

	return oldestID, nil
}

// UpdateHeartbeat updates the last_active_at timestamp for a cell.
func (cm *CellManager) UpdateHeartbeat(ctx context.Context, cellID string) error {
	return cm.router.SetLastActive(ctx, cellID)
}

// hostForRoute returns a gRPC client for the host in the route.
// MVP: single host, port 50051. Later: look up from host registry.
func (cm *CellManager) hostForRoute(route *router.CellRoute) (*hostclient.Client, error) {
	return cm.hosts.Get("host-1", route.Host, 50051)
}
