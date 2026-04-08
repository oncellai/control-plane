package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/oncellai/control-plane/internal/cellmanager"
	"github.com/oncellai/control-plane/internal/router"
	"github.com/oncellai/control-plane/internal/scheduler"
)

type Handler struct {
	cm     *cellmanager.CellManager
	router *router.Router
	sched  *scheduler.Scheduler
}

func NewHandler(cm *cellmanager.CellManager, r *router.Router, s *scheduler.Scheduler) http.Handler {
	h := &Handler{cm: cm, router: r, sched: s}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /cells/create", h.createCell)
	mux.HandleFunc("POST /cells/pause", h.pauseCell)
	mux.HandleFunc("POST /cells/resume", h.resumeCell)
	mux.HandleFunc("POST /cells/delete", h.deleteCell)
	mux.HandleFunc("GET /cells/status/{id}", h.cellStatus)
	mux.HandleFunc("GET /cells/route/{customer_id}", h.cellRoute)
	mux.HandleFunc("POST /cells/heartbeat/{id}", h.heartbeat)
	mux.HandleFunc("POST /cells/reclaim", h.forceReclaim)
	mux.HandleFunc("GET /cells/counts", h.cellCounts)
	mux.HandleFunc("POST /hosts/register", h.registerHost)
	mux.HandleFunc("POST /cells/set-permanent", h.setPermanent)

	return mux
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) createCell(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CellID      string            `json:"cell_id"`
		CustomerID  string            `json:"customer_id"`
		DeveloperID string            `json:"developer_id"`
		Permanent   bool              `json:"permanent"`
		Image       string            `json:"image"`
		Secrets     map[string]string `json:"secrets"`
		Spec        *struct {
			CPUMillicores int32 `json:"cpu_millicores"`
			MemoryMB      int32 `json:"memory_mb"`
			StorageGB     int32 `json:"storage_gb"`
		} `json:"spec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}

	// Default spec if not provided
	cpuMillicores := int32(1000)
	memoryMB := int32(1024)
	storageGB := int32(10)
	if req.Spec != nil {
		if req.Spec.CPUMillicores > 0 { cpuMillicores = req.Spec.CPUMillicores }
		if req.Spec.MemoryMB > 0 { memoryMB = req.Spec.MemoryMB }
		if req.Spec.StorageGB > 0 { storageGB = req.Spec.StorageGB }
	}

	image := req.Image
	if image == "" {
		image = "default"
	}

	result, err := h.cm.Create(r.Context(), req.CellID, req.CustomerID, req.DeveloperID, cpuMillicores, memoryMB, storageGB, image, req.Permanent, req.Secrets)
	if err != nil {
		slog.Error("create cell failed", "err", err)
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 201, result)
}

func (h *Handler) pauseCell(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CellID string `json:"cell_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}

	if err := h.cm.Pause(r.Context(), req.CellID); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]string{"cell_id": req.CellID, "status": "paused"})
}

func (h *Handler) resumeCell(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CellID string `json:"cell_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}

	result, err := h.cm.Resume(r.Context(), req.CellID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, result)
}

func (h *Handler) deleteCell(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CellID string `json:"cell_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}

	if err := h.cm.Delete(r.Context(), req.CellID); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) cellStatus(w http.ResponseWriter, r *http.Request) {
	cellID := r.PathValue("id")
	route, err := h.cm.GetRoute(r.Context(), cellID)
	if err != nil || route == nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, route)
}

func (h *Handler) cellRoute(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customer_id")
	route, err := h.router.GetRoute(r.Context(), customerID)
	if err != nil || route == nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, route)
}

func (h *Handler) heartbeat(w http.ResponseWriter, r *http.Request) {
	cellID := r.PathValue("id")
	if err := h.cm.UpdateHeartbeat(r.Context(), cellID); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) forceReclaim(w http.ResponseWriter, r *http.Request) {
	reclaimedID, err := h.cm.ForceReclaim(r.Context())
	if err != nil {
		writeJSON(w, 409, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"reclaimed_cell_id": reclaimedID})
}

func (h *Handler) setPermanent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CellID    string `json:"cell_id"`
		Permanent bool   `json:"permanent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}

	route, err := h.router.GetRoute(r.Context(), req.CellID)
	if err != nil || route == nil {
		writeJSON(w, 404, map[string]string{"error": "cell not found"})
		return
	}

	route.Permanent = req.Permanent
	if err := h.router.SetRoute(r.Context(), req.CellID, *route); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	slog.Info("cell permanent flag updated", "cell_id", req.CellID, "permanent", req.Permanent)
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) registerHost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HostID   string `json:"host_id"`
		Address  string `json:"address"`
		GRPCPort int    `json:"grpc_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}

	if req.HostID == "" || req.Address == "" {
		writeJSON(w, 400, map[string]string{"error": "host_id and address required"})
		return
	}
	if req.GRPCPort == 0 {
		req.GRPCPort = 50051
	}

	h.sched.RegisterHost(req.HostID, req.Address, req.GRPCPort)
	slog.Info("host registered", "host_id", req.HostID, "address", req.Address, "port", req.GRPCPort)
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

// cellCounts returns active/paused cell counts per developer for billing metering.
func (h *Handler) cellCounts(w http.ResponseWriter, r *http.Request) {
	counts, err := h.router.GetCellCountsByDeveloper(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, counts)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
