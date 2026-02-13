package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad_ValidFull(t *testing.T) {
	f, err := os.Open("testdata/valid_full.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build assertions
	if cfg.Build == nil {
		t.Fatal("expected build config")
	}
	if len(cfg.Build.Steps) != 2 {
		t.Fatalf("expected 2 build steps, got %d", len(cfg.Build.Steps))
	}
	if cfg.Build.Steps[0].Name != "go-build" {
		t.Errorf("expected step name 'go-build', got %q", cfg.Build.Steps[0].Name)
	}

	// Test assertions
	if cfg.Test == nil {
		t.Fatal("expected test config")
	}
	if len(cfg.Test.Steps) != 2 {
		t.Fatalf("expected 2 test steps, got %d", len(cfg.Test.Steps))
	}
	if !cfg.Test.Steps[1].Parallel {
		t.Error("expected helm-lint step to be parallel")
	}

	// Deploy assertions
	if cfg.Deploy == nil {
		t.Fatal("expected deploy config")
	}
	if cfg.Deploy.Artifact != "helm_chart" {
		t.Errorf("expected deploy artifact 'helm_chart', got %q", cfg.Deploy.Artifact)
	}
}

func TestLoad_ValidBuildOnly(t *testing.T) {
	f, err := os.Open("testdata/valid_build_only.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Build == nil {
		t.Fatal("expected build config")
	}
	if cfg.Test != nil {
		t.Error("expected test config to be nil")
	}
	if cfg.Deploy != nil {
		t.Error("expected deploy config to be nil")
	}
}

func TestLoad_InvalidMissingName(t *testing.T) {
	f, err := os.Open("testdata/invalid_missing_name.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, err = Load(f)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestLoad_InvalidBadOutputType(t *testing.T) {
	f, err := os.Open("testdata/invalid_bad_output_type.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, err = Load(f)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestLoad_EmptyInput(t *testing.T) {
	_, err := Load(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}
