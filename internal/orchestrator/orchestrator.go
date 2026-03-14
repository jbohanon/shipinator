package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"git.nonahob.net/jacob/shipinator/internal/buildinator"
	shipcfg "git.nonahob.net/jacob/shipinator/internal/config"
	"git.nonahob.net/jacob/shipinator/internal/deployinator"
	"git.nonahob.net/jacob/shipinator/internal/executor"
	"git.nonahob.net/jacob/shipinator/internal/store"
	"git.nonahob.net/jacob/shipinator/internal/testinator"
)

const defaultPollInterval = 5 * time.Second

// Config holds Orchestrator-level operational configuration.
type Config struct {
	// BuilderImage is the container image used to execute build and test steps.
	BuilderImage string
	// PollInterval controls how often the orchestrator polls the executor for
	// step completion. Defaults to 5 seconds if zero.
	PollInterval time.Duration
	// ArtifactBackend is the storage backend label recorded in artifact
	// metadata (e.g. artifact.BackendNFS).
	ArtifactBackend string
	// ArtifactBasePath is the root path on the artifact backend where the
	// executor writes artifact bytes (e.g. "/artifacts").
	ArtifactBasePath string
}

// Orchestrator drives pipeline runs from pending through to a terminal state.
// Stage-specific logic is delegated to the buildinator, testinator, and
// deployinator subsystems; the orchestrator manages job lifecycle and FSM
// transitions.
type Orchestrator struct {
	runs       store.PipelineRunStore
	jobs       store.JobStore
	steps      store.JobStepStore
	executions store.ExecutionStore
	exec       executor.Executor

	build  *buildinator.Buildinator
	test   *testinator.Testinator
	deploy *deployinator.Deployinator

	pollInterval time.Duration
}

// New creates a new Orchestrator. artifacts is the DB store used to register
// and look up artifact metadata across build and deploy stages.
func New(
	runs store.PipelineRunStore,
	jobs store.JobStore,
	steps store.JobStepStore,
	executions store.ExecutionStore,
	artifacts store.ArtifactStore,
	exec executor.Executor,
	cfg Config,
) *Orchestrator {
	pi := cfg.PollInterval
	if pi == 0 {
		pi = defaultPollInterval
	}
	o := &Orchestrator{
		runs:         runs,
		jobs:         jobs,
		steps:        steps,
		executions:   executions,
		exec:         exec,
		pollInterval: pi,
	}
	o.build = buildinator.New(
		buildinator.StepFn(o.runStep),
		artifacts,
		cfg.ArtifactBackend,
		cfg.ArtifactBasePath,
		cfg.BuilderImage,
	)
	o.test = testinator.New(testinator.StepFn(o.runStep), cfg.BuilderImage)
	o.deploy = deployinator.New(deployinator.StepFn(o.runStep), artifacts, cfg.BuilderImage)
	return o
}

// Run executes the pipeline run identified by runID using the provided config.
// It blocks until the run reaches a terminal state or ctx is cancelled.
func (o *Orchestrator) Run(ctx context.Context, runID store.PipelineRunID, cfg *shipcfg.ShipinatorConfig) error {
	log := slog.With("run_id", runID)

	if err := o.runs.UpdateStatus(ctx, runID, string(PipelineRunStateRunning)); err != nil {
		return fmt.Errorf("start pipeline run: %w", err)
	}
	log.Info("pipeline run started")

	runErr := o.executeStages(ctx, runID, cfg, log)

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

func (o *Orchestrator) executeStages(ctx context.Context, runID store.PipelineRunID, cfg *shipcfg.ShipinatorConfig, log *slog.Logger) error {
	var buildJobID store.JobID

	if cfg.Build != nil {
		id, err := o.runStage(ctx, runID, store.JobTypeBuild, func(ctx context.Context, jobID store.JobID) error {
			return o.build.Run(ctx, jobID, cfg.Build.Steps)
		})
		if err != nil {
			log.Error("build stage failed", "error", err)
			return err
		}
		buildJobID = id
	}

	if cfg.Test != nil {
		if _, err := o.runStage(ctx, runID, store.JobTypeTest, func(ctx context.Context, jobID store.JobID) error {
			return o.test.Run(ctx, jobID, cfg.Test.Steps)
		}); err != nil {
			log.Error("test stage failed", "error", err)
			return err
		}
	}

	if cfg.Deploy != nil {
		if _, err := o.runStage(ctx, runID, store.JobTypeDeploy, func(ctx context.Context, jobID store.JobID) error {
			return o.deploy.Run(ctx, jobID, buildJobID, cfg.Deploy)
		}); err != nil {
			log.Error("deploy stage failed", "error", err)
			return err
		}
	}

	return nil
}

// runStage creates a job for the given stage type, transitions it through its
// lifecycle, and invokes fn to execute the stage's steps. It returns the job
// ID so callers can reference the job's artifacts in subsequent stages.
func (o *Orchestrator) runStage(ctx context.Context, runID store.PipelineRunID, jobType string, fn func(context.Context, store.JobID) error) (store.JobID, error) {
	log := slog.With("run_id", runID, "stage", jobType)

	job := &store.Job{
		PipelineRunID: runID,
		JobType:       jobType,
		Name:          jobType,
	}
	if err := o.jobs.Create(ctx, job); err != nil {
		return store.JobID{}, fmt.Errorf("create %s job: %w", jobType, err)
	}
	log = log.With("job_id", job.ID)

	if err := o.jobs.UpdateStatus(ctx, job.ID, string(StageStateRunning)); err != nil {
		return store.JobID{}, fmt.Errorf("start %s job: %w", jobType, err)
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

	return job.ID, stageErr
}

// runStep encapsulates the full lifecycle of a single job step:
// create → pending → running → submit → poll → terminal.
func (o *Orchestrator) runStep(ctx context.Context, jobID store.JobID, name string, order int, parallelGroup *string, spec executor.ExecutionSpec) error {
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
func (o *Orchestrator) executeAndPoll(ctx context.Context, stepID store.JobStepID, spec executor.ExecutionSpec, log *slog.Logger) error {
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
func (o *Orchestrator) poll(ctx context.Context, execID store.ExecutionID, handle executor.ExecutionHandle, log *slog.Logger) error {
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
