//go:build integration

package kubernetes_test

import (
	"context"
	"testing"
	"time"

	"git.nonahob.net/jacob/shipinator/internal/executor"
	kubexec "git.nonahob.net/jacob/shipinator/internal/executor/kubernetes"
)

// pollUntilTerminal polls Status every second until the execution reaches a
// terminal phase or ctx expires, logging each observed phase.
func pollUntilTerminal(ctx context.Context, t *testing.T, exec *kubexec.Executor, handle executor.ExecutionHandle) executor.ExecutionStatus {
	t.Helper()
	for {
		status, err := exec.Status(ctx, handle)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		t.Logf("  phase: %s", status.Phase)
		if status.IsTerminal() {
			return status
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for terminal status")
		case <-time.After(time.Second):
		}
	}
}

func TestIntegration_HappyPath(t *testing.T) {
	exec := kubexec.New(testClient, testNamespace)

	spec := executor.ExecutionSpec{
		Image:   "busybox:latest",
		Command: []string{"sh", "-c", "echo hello"},
	}
	t.Logf("submitting: image=%s command=%v", spec.Image, spec.Command)

	handle, err := exec.Submit(context.Background(), spec)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	t.Logf("submitted job: %s", handle.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	t.Log("polling for terminal status...")
	status := pollUntilTerminal(ctx, t, exec, handle)
	t.Logf("terminal: phase=%s finishedAt=%v", status.Phase, status.FinishedAt)

	if status.Phase != executor.ExecutionPhaseSucceeded {
		t.Errorf("phase: got %q, want succeeded", status.Phase)
	}
	if status.FinishedAt == nil {
		t.Error("expected FinishedAt to be set")
	}
}

func TestIntegration_FailingJob(t *testing.T) {
	exec := kubexec.New(testClient, testNamespace)

	spec := executor.ExecutionSpec{
		Image:   "busybox:latest",
		Command: []string{"sh", "-c", "exit 1"},
	}
	t.Logf("submitting: image=%s command=%v", spec.Image, spec.Command)

	handle, err := exec.Submit(context.Background(), spec)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	t.Logf("submitted job: %s", handle.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	t.Log("polling for terminal status...")
	status := pollUntilTerminal(ctx, t, exec, handle)
	t.Logf("terminal: phase=%s", status.Phase)

	if status.Phase != executor.ExecutionPhaseFailed {
		t.Errorf("phase: got %q, want failed", status.Phase)
	}
}

func TestIntegration_Cancel(t *testing.T) {
	exec := kubexec.New(testClient, testNamespace)

	spec := executor.ExecutionSpec{
		Image:   "busybox:latest",
		Command: []string{"sleep", "300"},
	}
	t.Logf("submitting long-running job: image=%s command=%v", spec.Image, spec.Command)

	handle, err := exec.Submit(context.Background(), spec)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	t.Logf("submitted job: %s", handle.ID)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	t.Log("waiting for job to reach running...")
	for {
		status, err := exec.Status(ctx, handle)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		t.Logf("  phase: %s", status.Phase)
		if status.Phase == executor.ExecutionPhaseRunning {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for job to start running")
		case <-time.After(time.Second):
		}
	}

	t.Logf("canceling job: %s", handle.ID)
	if err := exec.Cancel(context.Background(), handle); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	t.Log("job canceled")
}

func TestIntegration_Timeout(t *testing.T) {
	exec := kubexec.New(testClient, testNamespace)

	spec := executor.ExecutionSpec{
		Image:   "busybox:latest",
		Command: []string{"sleep", "300"},
		Timeout: 5 * time.Second,
	}
	t.Logf("submitting job with %s timeout: image=%s command=%v", spec.Timeout, spec.Image, spec.Command)

	handle, err := exec.Submit(context.Background(), spec)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	t.Logf("submitted job: %s", handle.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	t.Log("polling for terminal status (expect failed after timeout)...")
	status := pollUntilTerminal(ctx, t, exec, handle)
	t.Logf("terminal: phase=%s", status.Phase)

	if status.Phase != executor.ExecutionPhaseFailed {
		t.Errorf("phase: got %q, want failed (timeout kills job)", status.Phase)
	}
}
