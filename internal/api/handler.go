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
	mux.HandleFunc("GET /cells/{id}/status", h.cellStatus)
	mux.HandleFunc("GET /cells/route/{customer_id}", h.cellRoute)

	return mux
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) createCell(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CellID      string `json:"cell_id"`
		CustomerID  string `json:"customer_id"`
		DeveloperID string `json:"developer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}

	result, err := h.cm.Create(r.Context(), req.CellID, req.CustomerID, req.DeveloperID)
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
