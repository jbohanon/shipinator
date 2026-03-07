package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	shipcfg "git.nonahob.net/jacob/shipinator/internal/config"
	"git.nonahob.net/jacob/shipinator/internal/executor"
	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
)

const defaultPollInterval = 5 * time.Second

// Config holds Orchestrator-level operational configuration.
type Config struct {
	// BuilderImage is the container image used to execute build and test steps.
	BuilderImage string
	// PollInterval controls how often the orchestrator polls the executor for
	// step completion. Defaults to 5 seconds if zero.
	PollInterval time.Duration
}

// Orchestrator drives pipeline runs from pending through to a terminal state.
// It translates .shipinator.yaml configuration into executor submissions,
// manages FSM transitions, and persists all state changes.
type Orchestrator struct {
	runs       store.PipelineRunStore
	jobs       store.JobStore
	steps      store.JobStepStore
	executions store.ExecutionStore
	exec       executor.Executor

	builderImage string
	pollInterval time.Duration
}

// New creates a new Orchestrator.
func New(
	runs store.PipelineRunStore,
	jobs store.JobStore,
	steps store.JobStepStore,
	executions store.ExecutionStore,
	exec executor.Executor,
	cfg Config,
) *Orchestrator {
	pi := cfg.PollInterval
	if pi == 0 {
		pi = defaultPollInterval
	}
	return &Orchestrator{
		runs:         runs,
		jobs:         jobs,
		steps:        steps,
		executions:   executions,
		exec:         exec,
		builderImage: cfg.BuilderImage,
		pollInterval: pi,
	}
}

// Run executes the pipeline run identified by runID using the provided config.
// It blocks until the run reaches a terminal state or ctx is cancelled.
// The caller is responsible for ensuring the pipeline run exists in the store
// and is in the pending state before calling Run.
func (o *Orchestrator) Run(ctx context.Context, runID uuid.UUID, cfg *shipcfg.ShipinatorConfig) error {
	log := slog.With("run_id", runID)

	if err := o.runs.UpdateStatus(ctx, runID, string(PipelineRunStateRunning)); err != nil {
		return fmt.Errorf("start pipeline run: %w", err)
	}
	log.Info("pipeline run started")

	runErr := o.executeStages(ctx, runID, cfg, log)

	// Use a detached context for the final update so that context cancellation
	// does not prevent recording the terminal state.
	finalCtx := context.WithoutCancel(ctx)
	finalState := PipelineRunStateSuccess
	switch {
	case ctx.Err() != nil:
		finalState = PipelineRunStateCanceled
	case runErr != nil:
		finalState = PipelineRunStateFailed
	}

	if err := o.runs.UpdateStatus(finalCtx, runID, string(finalState)); err != nil {
		log.Error("failed to finalize pipeline run", "state", finalState, "error", err)
	}

	log.Info("pipeline run finished", "state", finalState)
	return runErr
}

func (o *Orchestrator) executeStages(ctx context.Context, runID uuid.UUID, cfg *shipcfg.ShipinatorConfig, log *slog.Logger) error {
	if cfg.Build != nil {
		if err := o.runStage(ctx, runID, "build", func(ctx context.Context, jobID uuid.UUID) error {
			return o.runBuildSteps(ctx, jobID, cfg.Build.Steps)
		}); err != nil {
			log.Error("build stage failed", "error", err)
			return err
		}
	}

	if cfg.Test != nil {
		if err := o.runStage(ctx, runID, "test", func(ctx context.Context, jobID uuid.UUID) error {
			return o.runTestSteps(ctx, jobID, cfg.Test.Steps)
		}); err != nil {
			log.Error("test stage failed", "error", err)
			return err
		}
	}

	if cfg.Deploy != nil {
		if err := o.runStage(ctx, runID, "deploy", func(ctx context.Context, jobID uuid.UUID) error {
			return o.runDeployStep(ctx, jobID, cfg.Deploy)
		}); err != nil {
			log.Error("deploy stage failed", "error", err)
			return err
		}
	}

	return nil
}

// runStage creates a job for the given stage type, transitions it through its
// lifecycle, and invokes fn to execute the stage's steps.
func (o *Orchestrator) runStage(ctx context.Context, runID uuid.UUID, jobType string, fn func(context.Context, uuid.UUID) error) error {
	log := slog.With("run_id", runID, "stage", jobType)

	job := &store.Job{
		PipelineRunID: runID,
		JobType:       jobType,
		Name:          jobType,
	}
	if err := o.jobs.Create(ctx, job); err != nil {
		return fmt.Errorf("create %s job: %w", jobType, err)
	}
	log = log.With("job_id", job.ID)

	if err := o.jobs.UpdateStatus(ctx, job.ID, string(StageStateRunning)); err != nil {
		return fmt.Errorf("start %s job: %w", jobType, err)
	}
	log.Info("stage started")

	stageErr := fn(ctx, job.ID)

	finalState := StageStateSucceeded
	if stageErr != nil {
		finalState = StageStateFailed
	}

	finalCtx := context.WithoutCancel(ctx)
	if err := o.jobs.UpdateStatus(finalCtx, job.ID, string(finalState)); err != nil {
		log.Error("failed to finalize job state", "state", finalState, "error", err)
	}
	log.Info("stage finished", "state", finalState)

	return stageErr
}

// runBuildSteps runs build steps sequentially.
func (o *Orchestrator) runBuildSteps(ctx context.Context, jobID uuid.UUID, steps []shipcfg.BuildStep) error {
	for i, s := range steps {
		order := i
		artifacts := make([]executor.ArtifactSpec, len(s.Outputs))
		for j, out := range s.Outputs {
			artifacts[j] = executor.ArtifactSpec{Type: out.Type, Path: out.Path}
		}
		spec := executor.ExecutionSpec{
			Image:     o.builderImage,
			Command:   []string{"sh", "-c", s.Run},
			Env:       map[string]string{},
			Artifacts: artifacts,
		}
		if err := o.runStep(ctx, jobID, s.Name, order, nil, spec); err != nil {
			return err
		}
	}
	return nil
}

// runTestSteps runs test steps in order. Adjacent steps with Parallel: true
// are batched and executed concurrently; sequential steps run one at a time.
func (o *Orchestrator) runTestSteps(ctx context.Context, jobID uuid.UUID, steps []shipcfg.TestStep) error {
	i := 0
	order := 0
	for i < len(steps) {
		if !steps[i].Parallel {
			s := steps[i]
			spec := executor.ExecutionSpec{
				Image:   o.builderImage,
				Command: []string{"sh", "-c", s.Run},
				Env:     map[string]string{},
			}
			if err := o.runStep(ctx, jobID, s.Name, order, nil, spec); err != nil {
				return err
			}
			i++
			order++
			continue
		}

		// Collect consecutive parallel steps into a batch.
		groupName := fmt.Sprintf("parallel-%d", order)
		var batch []shipcfg.TestStep
		for i < len(steps) && steps[i].Parallel {
			batch = append(batch, steps[i])
			i++
		}
		if err := o.runParallelTestSteps(ctx, jobID, batch, order, groupName); err != nil {
			return err
		}
		order += len(batch)
	}
	return nil
}

func (o *Orchestrator) runParallelTestSteps(ctx context.Context, jobID uuid.UUID, steps []shipcfg.TestStep, orderBase int, group string) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for i, s := range steps {
		wg.Add(1)
		go func(s shipcfg.TestStep, ord int) {
			defer wg.Done()
			spec := executor.ExecutionSpec{
				Image:   o.builderImage,
				Command: []string{"sh", "-c", s.Run},
				Env:     map[string]string{},
			}
			if err := o.runStep(ctx, jobID, s.Name, ord, &group, spec); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(s, orderBase+i)
	}
	wg.Wait()

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// runDeployStep submits a single deploy execution derived from DeployConfig.
// TODO: resolve artifact storage path from a prior build artifact once the
// executor callback API is implemented.
func (o *Orchestrator) runDeployStep(ctx context.Context, jobID uuid.UUID, cfg *shipcfg.DeployConfig) error {
	cmd := fmt.Sprintf("helm upgrade --install shipinator-%s . --namespace %s", cfg.Artifact, cfg.Namespace)
	spec := executor.ExecutionSpec{
		Image:   o.builderImage,
		Command: []string{"sh", "-c", cmd},
		Env:     map[string]string{},
	}
	return o.runStep(ctx, jobID, "deploy", 0, nil, spec)
}

// runStep encapsulates the full lifecycle of a single job step:
// create → pending → running → submit → poll → terminal.
func (o *Orchestrator) runStep(ctx context.Context, jobID uuid.UUID, name string, order int, parallelGroup *string, spec executor.ExecutionSpec) error {
	log := slog.With("job_id", jobID, "step", name)

	step := &store.JobStep{
		JobID:          jobID,
		Name:           name,
		ExecutionOrder: &order,
		ParallelGroup:  parallelGroup,
	}
	if err := o.steps.Create(ctx, step); err != nil {
		return fmt.Errorf("create step %q: %w", name, err)
	}
	log = log.With("step_id", step.ID)

	if err := o.steps.UpdateStatus(ctx, step.ID, string(StageStateRunning)); err != nil {
		return fmt.Errorf("start step %q: %w", name, err)
	}
	log.Info("step started")

	stepErr := o.executeAndPoll(ctx, step.ID, spec, log)

	finalState := StageStateSucceeded
	if stepErr != nil {
		finalState = StageStateFailed
	}

	finalCtx := context.WithoutCancel(ctx)
	if err := o.steps.UpdateStatus(finalCtx, step.ID, string(finalState)); err != nil {
		log.Error("failed to finalize step state", "state", finalState, "error", err)
	}
	log.Info("step finished", "state", finalState)

	return stepErr
}

// executeAndPoll submits a step to the executor, records the execution, and
// polls until a terminal state is reached or ctx is cancelled.
func (o *Orchestrator) executeAndPoll(ctx context.Context, stepID uuid.UUID, spec executor.ExecutionSpec, log *slog.Logger) error {
	handle, err := o.exec.Submit(ctx, spec)
	if err != nil {
		return fmt.Errorf("submit step: %w", err)
	}

	exec := &store.Execution{
		JobStepID:    stepID,
		ExecutorType: "kubernetes",
		ExternalID:   handle.ID,
		Status:       string(executor.ExecutionPhasePending),
	}
	if err := o.executions.Create(ctx, exec); err != nil {
		return fmt.Errorf("record execution: %w", err)
	}

	return o.poll(ctx, exec.ID, handle, log)
}

// poll waits until the executor reports a terminal status for handle,
// updating the execution record on each phase change.
func (o *Orchestrator) poll(ctx context.Context, execID uuid.UUID, handle executor.ExecutionHandle, log *slog.Logger) error {
	ticker := time.NewTicker(o.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = o.exec.Cancel(context.Background(), handle)
			return ctx.Err()

		case <-ticker.C:
			status, err := o.exec.Status(ctx, handle)
			if err != nil {
				log.Warn("poll error", "handle", handle.ID, "error", err)
				continue
			}

			if err := o.executions.UpdateStatus(ctx, execID, string(status.Phase)); err != nil {
				log.Warn("failed to update execution status", "error", err)
			}

			if !status.IsTerminal() {
				continue
			}

			log.Info("execution terminal", "phase", status.Phase)
			if status.Phase != executor.ExecutionPhaseSucceeded {
				return fmt.Errorf("execution %s: phase=%s", handle.ID, status.Phase)
			}
			return nil
		}
	}
}
