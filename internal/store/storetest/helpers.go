//go:build integration

package storetest

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"git.nonahob.net/jacob/golibs/datastores/sql/migrate"
	"git.nonahob.net/jacob/golibs/datastores/sql/postgres"
	"git.nonahob.net/jacob/shipinator/internal/store"
	pgstore "git.nonahob.net/jacob/shipinator/internal/store/postgres"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewTestPool creates a unique test database, runs migrations, and returns a
// connection pool. The database is dropped when the test completes.
func NewTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	cfg := &postgres.Config{
		Host:     envOrDefault("SHIPINATOR_TEST_DB_HOST", "localhost"),
		Port:     envOrDefault("SHIPINATOR_TEST_DB_PORT", "5432"),
		User:     envOrDefault("SHIPINATOR_TEST_DB_USER", "postgres"),
		Password: envOrDefault("SHIPINATOR_TEST_DB_PASSWORD", "postgres"),
		DBName:   "postgres",
		SSLMode:  envOrDefault("SHIPINATOR_TEST_DB_SSLMODE", "disable"),
	}

	dbName := "shipinator_test_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	quotedName := pgx.Identifier{dbName}.Sanitize()

	// Connect to the default database to create the test database.
	adminPool, err := postgres.NewPool(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("connecting to admin database: %v", err)
	}

	_, err = adminPool.Exec(context.Background(), "CREATE DATABASE "+quotedName)
	if err != nil {
		postgres.ClosePool(adminPool)
		t.Fatalf("creating test database %s: %v", dbName, err)
	}
	postgres.ClosePool(adminPool)

	// Run migrations against the test database.
	testCfg := &postgres.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		DBName:   dbName,
		SSLMode:  cfg.SSLMode,
	}

	if err := testCfg.Migrate(&migrate.Options{MigrationsDir: migrationsDir()}); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := postgres.NewPool(context.Background(), testCfg, nil)
	if err != nil {
		t.Fatalf("connecting to test database %s: %v", dbName, err)
	}

	t.Cleanup(func() {
		postgres.ClosePool(pool)

		cleanupCfg := &postgres.Config{
			Host:     cfg.Host,
			Port:     cfg.Port,
			User:     cfg.User,
			Password: cfg.Password,
			DBName:   "postgres",
			SSLMode:  cfg.SSLMode,
		}
		cleanupPool, err := postgres.NewPool(context.Background(), cleanupCfg, nil)
		if err != nil {
			t.Logf("WARNING: failed to connect for cleanup: %v", err)
			return
		}
		defer postgres.ClosePool(cleanupPool)

		_, err = cleanupPool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+quotedName)
		if err != nil {
			t.Logf("WARNING: failed to drop test database %s: %v", dbName, err)
		}
	})

	return pool
}

// EntityChain holds IDs for a full entity hierarchy created by CreateEntityChain.
type EntityChain struct {
	ProjectID     uuid.UUID
	RepositoryID  uuid.UUID
	PipelineID    uuid.UUID
	PipelineRunID uuid.UUID
	JobID         uuid.UUID
	JobStepID     uuid.UUID
}

// CreateEntityChain creates a full entity hierarchy (project -> repository ->
// pipeline -> pipeline_run -> job -> job_step) and returns all IDs. This
// satisfies FK constraints for testing child stores.
func CreateEntityChain(t *testing.T, pool *pgxpool.Pool) EntityChain {
	t.Helper()
	ctx := context.Background()

	projectStore := pgstore.NewProjectStore(pool)
	repoStore := pgstore.NewRepositoryStore(pool)
	pipelineStore := pgstore.NewPipelineStore(pool)
	runStore := pgstore.NewPipelineRunStore(pool)
	jobStore := pgstore.NewJobStore(pool)
	stepStore := pgstore.NewJobStepStore(pool)

	p := &store.Project{Name: "test-project-" + uuid.New().String()[:8]}
	if err := projectStore.Create(ctx, p); err != nil {
		t.Fatalf("creating project: %v", err)
	}

	r := &store.Repository{
		ProjectID:     p.ID,
		VCSProvider:   "git",
		CloneURL:      "https://example.com/repo.git",
		DefaultBranch: "main",
	}
	if err := repoStore.Create(ctx, r); err != nil {
		t.Fatalf("creating repository: %v", err)
	}

	pl := &store.Pipeline{
		RepositoryID: r.ID,
		Name:         "test-pipeline",
		TriggerType:  "push",
	}
	if err := pipelineStore.Create(ctx, pl); err != nil {
		t.Fatalf("creating pipeline: %v", err)
	}

	run := &store.PipelineRun{
		PipelineID: pl.ID,
		GitRef:     "refs/heads/main",
		GitSHA:     "abc123def456",
	}
	if err := runStore.Create(ctx, run); err != nil {
		t.Fatalf("creating pipeline run: %v", err)
	}

	j := &store.Job{
		PipelineRunID: run.ID,
		JobType:       "build",
		Name:          "test-job",
	}
	if err := jobStore.Create(ctx, j); err != nil {
		t.Fatalf("creating job: %v", err)
	}

	step := &store.JobStep{
		JobID: j.ID,
		Name:  "test-step",
	}
	if err := stepStore.Create(ctx, step); err != nil {
		t.Fatalf("creating job step: %v", err)
	}

	return EntityChain{
		ProjectID:     p.ID,
		RepositoryID:  r.ID,
		PipelineID:    pl.ID,
		PipelineRunID: run.ID,
		JobID:         j.ID,
		JobStepID:     step.ID,
	}
}

// migrationsDir returns the absolute path to the migrations directory,
// relative to the project root.
func migrationsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(projectRoot, "migrations")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
