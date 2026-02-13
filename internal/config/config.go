package config

import (
	"fmt"
	"io"
	"os"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

var validate = validator.New()

// ShipinatorConfig is the top-level configuration parsed from .shipinator.yaml.
// At least one of Build, Test, or Deploy must be defined.
type ShipinatorConfig struct {
	Build  *BuildConfig  `yaml:"build"  validate:"omitempty"`
	Test   *TestConfig   `yaml:"test"   validate:"omitempty"`
	Deploy *DeployConfig `yaml:"deploy" validate:"omitempty"`
}

type BuildConfig struct {
	Steps []BuildStep `yaml:"steps" validate:"required,min=1,dive"`
}

type BuildStep struct {
	Name    string        `yaml:"name"    validate:"required"`
	Run     string        `yaml:"run"     validate:"required"`
	Outputs []BuildOutput `yaml:"outputs" validate:"omitempty,dive"`
}

type BuildOutput struct {
	Type string `yaml:"type" validate:"required,oneof=binary oci_image helm_chart"`
	Path string `yaml:"path" validate:"required_if=Type binary,required_if=Type helm_chart"`
	Ref  string `yaml:"ref"  validate:"required_if=Type oci_image"`
}

type TestConfig struct {
	Steps []TestStep `yaml:"steps" validate:"required,min=1,dive"`
}

type TestStep struct {
	Name      string         `yaml:"name"      validate:"required"`
	Run       string         `yaml:"run"       validate:"required"`
	Parallel  bool           `yaml:"parallel"`
	Artifacts []TestArtifact `yaml:"artifacts" validate:"omitempty,dive"`
}

type TestArtifact struct {
	Type string `yaml:"type" validate:"required,oneof=coverage test_report"`
	Path string `yaml:"path" validate:"required"`
}

type DeployConfig struct {
	Artifact  string `yaml:"artifact"  validate:"required,oneof=binary oci_image helm_chart"`
	Target    string `yaml:"target"    validate:"required"`
	Namespace string `yaml:"namespace" validate:"required"`
}

// Load reads YAML from r, parses it into ShipinatorConfig, and validates.
func Load(r io.Reader) (*ShipinatorConfig, error) {
	var cfg ShipinatorConfig
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Build == nil && cfg.Test == nil && cfg.Deploy == nil {
		return nil, fmt.Errorf("config must define at least one of: build, test, deploy")
	}

	if err := validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// LoadFromFile reads and parses a .shipinator.yaml file at the given path.
func LoadFromFile(path string) (*ShipinatorConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening config file: %w", err)
	}
	defer f.Close()
	return Load(f)
}
