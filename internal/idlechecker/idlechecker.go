package idlechecker

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/oncellai/control-plane/internal/cellmanager"
	"github.com/oncellai/control-plane/internal/router"
)

// Tier-based idle timeouts. The cell's tier is embedded in the route metadata.
// Free tier cells get evicted aggressively to minimize cost at scale.
var tierTimeouts = map[string]time.Duration{
	"free":        5 * time.Minute,
	"starter":     30 * time.Minute,
	"standard":    2 * time.Hour,
	"performance": 6 * time.Hour,
}

type IdleChecker struct {
	cm             *cellmanager.CellManager
	router         *router.Router
	defaultTimeout time.Duration
}

func New(cm *cellmanager.CellManager, r *router.Router, timeout time.Duration) *IdleChecker {
	return &IdleChecker{cm: cm, router: r, defaultTimeout: timeout}
}

// Run checks for idle cells every 30 seconds.
func (ic *IdleChecker) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	slog.Info("idle checker started", "default_timeout", ic.defaultTimeout, "tier_timeouts", tierTimeouts)

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

		// Use tier-specific timeout if available
		timeout := ic.getTimeoutForCell(route)

		idle := time.Duration(now-lastActive) * time.Second
		if idle > timeout {
			slog.Info("idle checker: pausing cell", "cell_id", cellID, "idle", idle, "timeout", timeout)
			if err := ic.cm.Pause(ctx, cellID); err != nil {
				slog.Error("idle checker: failed to pause", "cell_id", cellID, "err", err)
			}
		}
	}
}

// getTimeoutForCell returns the idle timeout based on the cell's tier.
func (ic *IdleChecker) getTimeoutForCell(route *router.CellRoute) time.Duration {
	if route == nil || route.Tier == "" {
		return ic.defaultTimeout
	}

	tier := strings.ToLower(route.Tier)
	if timeout, ok := tierTimeouts[tier]; ok {
		return timeout
	}

	return ic.defaultTimeout
}
