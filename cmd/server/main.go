// Command server runs the TickFlow API: config loading, structured logging,
// database migration and the HTTP router.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LinuxSuRen/tick-flow/internal/config"
	"github.com/LinuxSuRen/tick-flow/internal/repository"
	"github.com/LinuxSuRen/tick-flow/internal/router"
	"github.com/LinuxSuRen/tick-flow/migrations"
)

// version is overridden at build time (-X main.version) by the release
// pipeline; development builds keep "dev".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	db, err := repository.Open(cfg.DBDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := repository.Migrate(db, migrations.FS); err != nil {
		return err
	}
	slog.Info("database ready")

	engine := router.New(router.Deps{
		DB:        db,
		JWTSecret: cfg.JWTSecret,
	})

	slog.Info("starting tickflow server", "version", version)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: engine,
	}

	srvErr := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", cfg.Addr)
		srvErr <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-srvErr:
		return err
	case <-stop:
		slog.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}
