package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"git.nonahob.net/jacob/golibs/datastores/sql/migrate"
	"git.nonahob.net/jacob/golibs/datastores/sql/postgres"
	"git.nonahob.net/jacob/shipinator/internal/artifact"
	artifactfs "git.nonahob.net/jacob/shipinator/internal/artifact/filesystem"
	apiserver "git.nonahob.net/jacob/shipinator/internal/server"
	"git.nonahob.net/jacob/shipinator/internal/server/config"
	pgstore "git.nonahob.net/jacob/shipinator/internal/store/postgres"
	"github.com/labstack/echo/v4"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		slog.Error("invalid log level", "level", cfg.LogLevel, "error", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Info("config loaded",
		"listen_addr", cfg.ListenAddr,
		"db_name", cfg.DB.Name,
		"artifact_path", cfg.ArtifactPath,
		"kubeconfig", cfg.KubeConfig,
		"log_level", cfg.LogLevel,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	pgCfg := cfg.DB.PostgresConfig()

	slog.Info("running database migrations")
	if err := pgCfg.Migrate(&migrate.Options{MigrationsDir: "migrations"}); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations complete")

	pool, err := postgres.NewPool(ctx, pgCfg, nil)
	if err != nil {
		slog.Error("failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer postgres.ClosePool(pool)

	// Construct store layer.
	pipelineStore := pgstore.NewPipelineStore(pool)
	pipelineRunStore := pgstore.NewPipelineRunStore(pool)
	jobStore := pgstore.NewJobStore(pool)
	artifactStore := pgstore.NewArtifactStore(pool)
	executionStore := pgstore.NewExecutionStore(pool)

	artifactBytesStore := artifactfs.New(artifact.BackendNFS, cfg.ArtifactPath)
	api := apiserver.New(
		pipelineStore,
		pipelineRunStore,
		jobStore,
		artifactStore,
		executionStore,
		artifactBytesStore,
	)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	api.RegisterRoutes(e)

	go func() {
		slog.Info("server starting", "addr", cfg.ListenAddr)
		if err := e.Start(cfg.ListenAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")
	if err := e.Shutdown(context.Background()); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
