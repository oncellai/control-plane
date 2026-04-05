package cellmanager

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/oncellai/control-plane/internal/router"
	"github.com/oncellai/control-plane/internal/scheduler"
)

type CellManager struct {
	router    *router.Router
	scheduler *scheduler.Scheduler
}

func New(r *router.Router, s *scheduler.Scheduler) *CellManager {
	return &CellManager{router: r, scheduler: s}
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
		return nil, fmt.Errorf("scheduler: %w", err)
	}

	// TODO: gRPC call to Host Agent to create the cell
	// For now, just register the route
	port := 8400 // TODO: get from Host Agent response

	route := router.CellRoute{
		Host:   host.Address,
		Port:   port,
		Status: "active",
		CellID: cellID,
	}

	if err := cm.router.SetRoute(ctx, cellID, route); err != nil {
		return nil, fmt.Errorf("set route: %w", err)
	}

	cm.router.SetLastActive(ctx, cellID)

	slog.Info("cell created", "cell_id", cellID, "host", host.ID, "port", port)

	return &CreateResult{
		CellID: cellID,
		HostID: host.ID,
		Port:   port,
		Status: "active",
	}, nil
}

func (cm *CellManager) Pause(ctx context.Context, cellID string) error {
	route, err := cm.router.GetRoute(ctx, cellID)
	if err != nil || route == nil {
		return fmt.Errorf("cell not found: %s", cellID)
	}

	// TODO: gRPC call to Host Agent to pause (snapshot + stop)

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

	// TODO: gRPC call to Host Agent to resume

	route.Status = "active"
	cm.router.SetRoute(ctx, cellID, *route)
	cm.router.SetLastActive(ctx, cellID)

	slog.Info("cell resumed", "cell_id", cellID)

	return &CreateResult{
		CellID: cellID,
		HostID: "host-1", // TODO: from route
		Port:   route.Port,
		Status: "active",
	}, nil
}

func (cm *CellManager) Delete(ctx context.Context, cellID string) error {
	// TODO: gRPC call to Host Agent to delete

	cm.router.DeleteRoute(ctx, cellID)

	slog.Info("cell deleted", "cell_id", cellID)
	return nil
}

func (cm *CellManager) GetRoute(ctx context.Context, cellID string) (*router.CellRoute, error) {
	return cm.router.GetRoute(ctx, cellID)
}
