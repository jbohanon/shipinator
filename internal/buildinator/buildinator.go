package buildinator

import (
	"context"
	"fmt"
	"path/filepath"

	shipcfg "git.nonahob.net/jacob/shipinator/internal/config"
	"git.nonahob.net/jacob/shipinator/internal/executor"
	"git.nonahob.net/jacob/shipinator/internal/store"
)

// StepFn executes a single pipeline step and is provided by the orchestrator.
type StepFn func(ctx context.Context, jobID store.JobID, name string, order int, group *string, spec executor.ExecutionSpec) error

// Buildinator translates build configuration into executor submissions and
// registers produced artifact metadata in the store on success.
type Buildinator struct {
	step         StepFn
	artifacts    store.ArtifactStore
	backend      string
	basePath     string
	builderImage string
}

// New creates a Buildinator. step is called for each build step; artifacts is
// the DB store where produced artifact metadata is persisted after each step
// succeeds. backend and basePath describe where the executor will write
// artifact bytes (e.g. "nfs", "/artifacts").
func New(step StepFn, artifacts store.ArtifactStore, backend, basePath, builderImage string) *Buildinator {
	return &Buildinator{
		step:         step,
		artifacts:    artifacts,
		backend:      backend,
		basePath:     basePath,
		builderImage: builderImage,
	}
}

// Run executes the build steps sequentially. After each step succeeds,
// artifact metadata is registered in the store for every declared output.
func (b *Buildinator) Run(ctx context.Context, jobID store.JobID, steps []shipcfg.BuildStep) error {
	for i, s := range steps {
		// Pre-allocate IDs so the storage locator is known before execution.
		ids := make([]store.ArtifactID, len(s.Outputs))
		for j := range s.Outputs {
			ids[j] = store.NewArtifactID()
		}

		spec := executor.ExecutionSpec{
			Image:     b.builderImage,
			Command:   []string{"sh", "-c", s.Run},
			Env:       map[string]string{},
			Artifacts: outputsToSpecs(s.Outputs),
		}

		if err := b.step(ctx, jobID, s.Name, i, nil, spec); err != nil {
			return err
		}

		for j, out := range s.Outputs {
			art := &store.Artifact{
				ID:             ids[j],
				JobID:          jobID,
				ArtifactType:   out.Type,
				StorageBackend: b.backend,
				StoragePath:    filepath.Join(b.basePath, ids[j].String()),
			}
			if err := b.artifacts.Create(ctx, art); err != nil {
				return fmt.Errorf("register artifact %q for step %q: %w", out.Type, s.Name, err)
			}
		}
	}
	return nil
}

func outputsToSpecs(outputs []shipcfg.BuildOutput) []executor.ArtifactSpec {
	specs := make([]executor.ArtifactSpec, len(outputs))
	for i, o := range outputs {
		specs[i] = executor.ArtifactSpec{Type: o.Type, Path: o.Path}
	}
	return specs
}
