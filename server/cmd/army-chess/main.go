package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fenghangyu/army-chess/server/internal/httpapi"
	"github.com/fenghangyu/army-chess/server/internal/persistence"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	db, err := persistence.Open(dbCtx, os.Getenv("DATABASE_URL"))
	cancel()
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	if db != nil {
		defer db.Close()
	}

	staticRoot := os.Getenv("WEB_ROOT")
	if staticRoot == "" {
		staticRoot = filepath.Join("web", "dist")
	}
	app := httpapi.New(logger, db, http.FileServer(http.Dir(staticRoot)))
	if db != nil {
		recoverCtx, recoverCancel := context.WithTimeout(ctx, 10*time.Second)
		if err := app.Rooms.Recover(recoverCtx); err != nil {
			recoverCancel()
			logger.Error("recover rooms", "error", err)
			os.Exit(1)
		}
		recoverCancel()
	}
	go tickRooms(ctx, app)

	server := &http.Server{
		Addr:              ":" + envOr("PORT", "8080"),
		Handler:           app.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", server.Addr, "web_root", staticRoot)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown", "error", err)
			os.Exit(1)
		}
	}
}

func tickRooms(ctx context.Context, app *httpapi.Server) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			app.Rooms.Tick(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
