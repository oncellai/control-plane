package idlechecker

import (
	"context"
	"log/slog"
	"time"

	"github.com/oncellai/control-plane/internal/cellmanager"
	"github.com/oncellai/control-plane/internal/router"
)

type IdleChecker struct {
	cm      *cellmanager.CellManager
	router  *router.Router
	timeout time.Duration
}

func New(cm *cellmanager.CellManager, r *router.Router, timeout time.Duration) *IdleChecker {
	return &IdleChecker{cm: cm, router: r, timeout: timeout}
}

// Run checks for idle cells every 30 seconds.
func (ic *IdleChecker) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	slog.Info("idle checker started", "timeout", ic.timeout)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ic.check(ctx)
		}
	}
}

func (ic *IdleChecker) check(ctx context.Context) {
	cells, err := ic.router.GetActiveCells(ctx)
	if err != nil {
		slog.Error("idle checker: failed to list cells", "err", err)
		return
	}

	now := time.Now().Unix()

	for _, cellID := range cells {
		// Check if cell is permanent — never evict permanent cells
		route, routeErr := ic.router.GetRoute(ctx, cellID)
		if routeErr != nil {
			continue
		}
		if route != nil && route.Permanent {
			continue
		}

		lastActive, err := ic.router.GetLastActive(ctx, cellID)
		if err != nil {
			continue
		}

		idle := time.Duration(now-lastActive) * time.Second
		if idle > ic.timeout {
			slog.Info("idle checker: pausing cell", "cell_id", cellID, "idle", idle)
			if err := ic.cm.Pause(ctx, cellID); err != nil {
				slog.Error("idle checker: failed to pause", "cell_id", cellID, "err", err)
			}
		}
	}
}
