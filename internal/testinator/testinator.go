package testinator

import (
	"context"
	"fmt"
	"sync"

	shipcfg "git.nonahob.net/jacob/shipinator/internal/config"
	"git.nonahob.net/jacob/shipinator/internal/executor"
	"git.nonahob.net/jacob/shipinator/internal/store"
)

// StepFn executes a single pipeline step and is provided by the orchestrator.
type StepFn func(ctx context.Context, jobID store.JobID, name string, order int, group *string, spec executor.ExecutionSpec) error

// Testinator translates test configuration into executor submissions.
// Adjacent steps marked Parallel: true are batched and run concurrently;
// sequential steps run one at a time in declaration order.
type Testinator struct {
	step         StepFn
	builderImage string
}

// New creates a Testinator.
func New(step StepFn, builderImage string) *Testinator {
	return &Testinator{step: step, builderImage: builderImage}
}

// Run executes the test steps, respecting sequential/parallel groupings.
func (t *Testinator) Run(ctx context.Context, jobID store.JobID, steps []shipcfg.TestStep) error {
	i, order := 0, 0
	for i < len(steps) {
		if !steps[i].Parallel {
			s := steps[i]
			spec := t.specForStep(s)
			if err := t.step(ctx, jobID, s.Name, order, nil, spec); err != nil {
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
		if err := t.runParallel(ctx, jobID, batch, order, groupName); err != nil {
			return err
		}
		order += len(batch)
	}
	return nil
}

func (t *Testinator) runParallel(ctx context.Context, jobID store.JobID, steps []shipcfg.TestStep, orderBase int, group string) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for i, s := range steps {
		wg.Add(1)
		go func(s shipcfg.TestStep, ord int) {
			defer wg.Done()
			if err := t.step(ctx, jobID, s.Name, ord, &group, t.specForStep(s)); err != nil {
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

func (t *Testinator) specForStep(s shipcfg.TestStep) executor.ExecutionSpec {
	return executor.ExecutionSpec{
		Image:   t.builderImage,
		Command: []string{"sh", "-c", s.Run},
		Env:     map[string]string{},
	}
}
