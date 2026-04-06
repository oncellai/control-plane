package router

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type CellRoute struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
	CellID   string `json:"cell_id"`
}

type HostMetrics struct {
	CPUTotal        int      `json:"cpu_total"`
	CPUUsed         int      `json:"cpu_used"`
	RAMTotal        int64    `json:"ram_total"`
	RAMUsed         int64    `json:"ram_used"`
	ActiveCells     int      `json:"active_cells"`
	PausedCells     int      `json:"paused_cells"`
	CachedCustomers []string `json:"cached_customers"`
}

type Router struct {
	rdb *redis.Client
}

func New(redisURL string) *Router {
	rdb := redis.NewClient(&redis.Options{Addr: redisURL})
	return &Router{rdb: rdb}
}

func (r *Router) SetRoute(ctx context.Context, cellID string, route CellRoute) error {
	data, _ := json.Marshal(route)
	return r.rdb.Set(ctx, cellKey(cellID), data, 0).Err()
}

func (r *Router) GetRoute(ctx context.Context, cellID string) (*CellRoute, error) {
	data, err := r.rdb.Get(ctx, cellKey(cellID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var route CellRoute
	json.Unmarshal(data, &route)
	return &route, nil
}

func (r *Router) DeleteRoute(ctx context.Context, cellID string) error {
	return r.rdb.Del(ctx, cellKey(cellID)).Err()
}

func (r *Router) SetLastActive(ctx context.Context, cellID string) error {
	return r.rdb.Set(ctx, lastActiveKey(cellID), time.Now().Unix(), 0).Err()
}

func (r *Router) GetLastActive(ctx context.Context, cellID string) (int64, error) {
	return r.rdb.Get(ctx, lastActiveKey(cellID)).Int64()
}

func (r *Router) GetHostMetrics(ctx context.Context, hostID string) (*HostMetrics, error) {
	data, err := r.rdb.Get(ctx, hostMetricsKey(hostID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m HostMetrics
	json.Unmarshal(data, &m)
	return &m, nil
}

func (r *Router) GetActiveCells(ctx context.Context) ([]string, error) {
	// Scan for all cell routes where status = active
	var cellIDs []string
	iter := r.rdb.Scan(ctx, 0, "cell:*:route", 1000).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		data, err := r.rdb.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}
		var route CellRoute
		json.Unmarshal(data, &route)
		if route.Status == "active" {
			cellIDs = append(cellIDs, route.CellID)
		}
	}
	return cellIDs, iter.Err()
}

// DeveloperCounts holds active/paused cell counts for billing.
type DeveloperCounts struct {
	DeveloperID string `json:"developer_id"`
	Active      int    `json:"active"`
	Paused      int    `json:"paused"`
}

// GetCellCountsByDeveloper returns active/paused cell counts grouped by developer.
// Cell IDs encode developer: "dev-abc123--customer-id"
func (r *Router) GetCellCountsByDeveloper(ctx context.Context) ([]DeveloperCounts, error) {
	counts := map[string]*DeveloperCounts{} // developer_id → counts

	iter := r.rdb.Scan(ctx, 0, "cell:*:route", 1000).Iterator()
	for iter.Next(ctx) {
		data, err := r.rdb.Get(ctx, iter.Val()).Bytes()
		if err != nil {
			continue
		}
		var route CellRoute
		json.Unmarshal(data, &route)

		// Extract developer_id from cell_id: "dev-abc123--customer-id"
		devID := route.CellID
		if idx := len(devID); idx > 0 {
			for i, c := range devID {
				if c == '-' && i > 0 && i+1 < len(devID) && devID[i+1] == '-' {
					devID = devID[:i]
					break
				}
			}
		}

		if counts[devID] == nil {
			counts[devID] = &DeveloperCounts{DeveloperID: devID}
		}
		if route.Status == "active" {
			counts[devID].Active++
		} else if route.Status == "paused" {
			counts[devID].Paused++
		}
	}

	result := make([]DeveloperCounts, 0, len(counts))
	for _, c := range counts {
		result = append(result, *c)
	}
	return result, iter.Err()
}

func cellKey(cellID string) string       { return fmt.Sprintf("cell:%s:route", cellID) }
func lastActiveKey(cellID string) string  { return fmt.Sprintf("cell:%s:last_active", cellID) }
func hostMetricsKey(hostID string) string { return fmt.Sprintf("host:%s:metrics", hostID) }
