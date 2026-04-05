package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oncellai/control-plane/internal/api"
	"github.com/oncellai/control-plane/internal/cellmanager"
	"github.com/oncellai/control-plane/internal/config"
	"github.com/oncellai/control-plane/internal/idlechecker"
	"github.com/oncellai/control-plane/internal/router"
	"github.com/oncellai/control-plane/internal/scheduler"
)

func main() {
	cfg := config.Load()

	slog.Info("starting oncell control plane",
		"port", cfg.Port,
		"redis", cfg.RedisURL,
	)

	// Initialize components
	rtr := router.New(cfg.RedisURL)
	sched := scheduler.New(rtr)
	cm := cellmanager.New(rtr, sched)

	// Start idle checker
	ic := idlechecker.New(cm, rtr, time.Duration(cfg.IdleTimeoutSecs)*time.Second)
	go ic.Run(context.Background())

	// Start HTTP server
	handler := api.NewHandler(cm, rtr, sched)
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	// Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		slog.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	slog.Info("listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
