package deployinator

import (
	"context"
	"fmt"

	shipcfg "git.nonahob.net/jacob/shipinator/internal/config"
	"git.nonahob.net/jacob/shipinator/internal/executor"
	"git.nonahob.net/jacob/shipinator/internal/store"
)

// StepFn executes a single pipeline step and is provided by the orchestrator.
type StepFn func(ctx context.Context, jobID store.JobID, name string, order int, group *string, spec executor.ExecutionSpec) error

// Deployinator translates deploy configuration into executor submissions
// using artifact references produced by a prior build stage. It never
// operates on source — only on pre-built artifact locators from the store.
type Deployinator struct {
	step         StepFn
	artifacts    store.ArtifactStore
	builderImage string
}

// New creates a Deployinator. artifacts is queried to resolve the artifact
// reference declared in the deploy config.
func New(step StepFn, artifacts store.ArtifactStore, builderImage string) *Deployinator {
	return &Deployinator{step: step, artifacts: artifacts, builderImage: builderImage}
}

// Run finds the artifact of the type declared in cfg from the build job
// identified by buildJobID, then submits a deploy execution against it.
// Returns an error if no matching artifact is found.
func (d *Deployinator) Run(ctx context.Context, jobID, buildJobID store.JobID, cfg *shipcfg.DeployConfig) error {
	arts, err := d.artifacts.ListByJob(ctx, buildJobID)
	if err != nil {
		return fmt.Errorf("list build artifacts: %w", err)
	}

	var art *store.Artifact
	for i := range arts {
		if arts[i].ArtifactType == cfg.Artifact {
			art = &arts[i]
			break
		}
	}
	if art == nil {
		return fmt.Errorf("no %q artifact found for build job %s", cfg.Artifact, buildJobID)
	}

	spec := executor.ExecutionSpec{
		Image:   d.builderImage,
		Command: []string{"sh", "-c", deployCmd(cfg, art)},
		Env:     map[string]string{},
	}
	return d.step(ctx, jobID, "deploy", 0, nil, spec)
}

func deployCmd(cfg *shipcfg.DeployConfig, art *store.Artifact) string {
	switch cfg.Artifact {
	case "helm_chart":
		return fmt.Sprintf("helm upgrade --install %s %s --namespace %s",
			cfg.Target, art.StoragePath, cfg.Namespace)
	default:
		return fmt.Sprintf("kubectl apply -f %s --namespace %s",
			art.StoragePath, cfg.Namespace)
	}
}
