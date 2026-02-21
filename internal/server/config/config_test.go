package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_FromYAML(t *testing.T) {
	yaml := `
listen_addr: ":9090"
db:
  host: "dbhost"
  port: "5433"
  user: "myuser"
  password: "mypass"
  name: "mydb"
  ssl_mode: "require"
artifact_path: "/tmp/artifacts"
kubeconfig: "/home/user/.kube/config"
log_level: "debug"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9090")
	}
	if cfg.DB.Host != "dbhost" {
		t.Errorf("DB.Host = %q, want %q", cfg.DB.Host, "dbhost")
	}
	if cfg.DB.Port != "5433" {
		t.Errorf("DB.Port = %q, want %q", cfg.DB.Port, "5433")
	}
	if cfg.DB.User != "myuser" {
		t.Errorf("DB.User = %q, want %q", cfg.DB.User, "myuser")
	}
	if cfg.DB.Password != "mypass" {
		t.Errorf("DB.Password = %q, want %q", cfg.DB.Password, "mypass")
	}
	if cfg.DB.Name != "mydb" {
		t.Errorf("DB.Name = %q, want %q", cfg.DB.Name, "mydb")
	}
	if cfg.DB.SSLMode != "require" {
		t.Errorf("DB.SSLMode = %q, want %q", cfg.DB.SSLMode, "require")
	}
	if cfg.ArtifactPath != "/tmp/artifacts" {
		t.Errorf("ArtifactPath = %q, want %q", cfg.ArtifactPath, "/tmp/artifacts")
	}
	if cfg.KubeConfig != "/home/user/.kube/config" {
		t.Errorf("KubeConfig = %q, want %q", cfg.KubeConfig, "/home/user/.kube/config")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	yaml := `
db:
  name: "fromyaml"
log_level: "warn"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SHIPINATOR_DB_NAME", "fromenv")
	t.Setenv("SHIPINATOR_LOG_LEVEL", "error")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DB.Name != "fromenv" {
		t.Errorf("DB.Name = %q, want %q", cfg.DB.Name, "fromenv")
	}
	if cfg.LogLevel != "error" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "error")
	}
}

func TestLoad_MissingDBName(t *testing.T) {
	cfg, err := Load("")
	if err == nil {
		t.Fatalf("expected error for missing db.name, got cfg=%+v", cfg)
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("SHIPINATOR_DB_NAME", "shipinator")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":8080")
	}
	if cfg.DB.Host != "localhost" {
		t.Errorf("DB.Host = %q, want %q", cfg.DB.Host, "localhost")
	}
	if cfg.DB.Port != "5432" {
		t.Errorf("DB.Port = %q, want %q", cfg.DB.Port, "5432")
	}
	if cfg.DB.User != "postgres" {
		t.Errorf("DB.User = %q, want %q", cfg.DB.User, "postgres")
	}
	if cfg.DB.Password != "postgres" {
		t.Errorf("DB.Password = %q, want %q", cfg.DB.Password, "postgres")
	}
	if cfg.DB.SSLMode != "disable" {
		t.Errorf("DB.SSLMode = %q, want %q", cfg.DB.SSLMode, "disable")
	}
	if cfg.ArtifactPath != "/var/lib/shipinator/artifacts" {
		t.Errorf("ArtifactPath = %q, want %q", cfg.ArtifactPath, "/var/lib/shipinator/artifacts")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.KubeConfig != "" {
		t.Errorf("KubeConfig = %q, want empty string", cfg.KubeConfig)
	}
}
