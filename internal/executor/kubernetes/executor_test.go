package kubernetes_test

import (
	"context"
	"testing"
	"time"

	"git.nonahob.net/jacob/shipinator/internal/executor"
	kubexec "git.nonahob.net/jacob/shipinator/internal/executor/kubernetes"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSubmit_CreatesJobWithCorrectSpec(t *testing.T) {
	client := fake.NewSimpleClientset()
	exec := kubexec.New(client, "test-ns")

	spec := executor.ExecutionSpec{
		Image:   "myimage:latest",
		Command: []string{"go", "test", "./..."},
		Env:     map[string]string{"FOO": "bar"},
		Timeout: 30 * time.Second,
	}
	t.Logf("submitting spec: image=%s command=%v timeout=%s", spec.Image, spec.Command, spec.Timeout)

	handle, err := exec.Submit(context.Background(), spec)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	t.Logf("got handle: %s", handle.ID)

	jobs, err := client.BatchV1().Jobs("test-ns").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs.Items) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs.Items))
	}

	job := jobs.Items[0]
	t.Logf("job created: name=%s namespace=%s", job.Name, job.Namespace)

	if job.Name != handle.ID {
		t.Errorf("handle ID %q does not match job name %q", handle.ID, job.Name)
	}

	container := job.Spec.Template.Spec.Containers[0]
	t.Logf("container: image=%s command=%v env=%v", container.Image, container.Command, container.Env)
	t.Logf("job spec: activeDeadlineSeconds=%v backoffLimit=%v restartPolicy=%s",
		job.Spec.ActiveDeadlineSeconds, job.Spec.BackoffLimit, job.Spec.Template.Spec.RestartPolicy)

	if container.Image != spec.Image {
		t.Errorf("image: got %q, want %q", container.Image, spec.Image)
	}
	if len(container.Command) != len(spec.Command) {
		t.Errorf("command: got %v, want %v", container.Command, spec.Command)
	}
	if job.Spec.ActiveDeadlineSeconds == nil || *job.Spec.ActiveDeadlineSeconds != 30 {
		t.Errorf("activeDeadlineSeconds: got %v, want 30", job.Spec.ActiveDeadlineSeconds)
	}

	envFound := false
	for _, e := range container.Env {
		if e.Name == "FOO" && e.Value == "bar" {
			envFound = true
		}
	}
	if !envFound {
		t.Error("env var FOO=bar not found in container spec")
	}

	if job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("RestartPolicy: got %q, want Never", job.Spec.Template.Spec.RestartPolicy)
	}
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 0 {
		t.Errorf("BackoffLimit: got %v, want 0", job.Spec.BackoffLimit)
	}
}

func TestSubmit_NoTimeout_OmitsActiveDeadline(t *testing.T) {
	client := fake.NewSimpleClientset()
	exec := kubexec.New(client, "test-ns")

	t.Log("submitting spec with no timeout")
	_, err := exec.Submit(context.Background(), executor.ExecutionSpec{
		Image:   "myimage:latest",
		Command: []string{"echo"},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	jobs, _ := client.BatchV1().Jobs("test-ns").List(context.Background(), metav1.ListOptions{})
	t.Logf("activeDeadlineSeconds: %v", jobs.Items[0].Spec.ActiveDeadlineSeconds)
	if jobs.Items[0].Spec.ActiveDeadlineSeconds != nil {
		t.Error("expected no ActiveDeadlineSeconds when Timeout is zero")
	}
}

func TestStatus_Pending(t *testing.T) {
	client := fake.NewSimpleClientset(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: "test-ns"},
	})
	exec := kubexec.New(client, "test-ns")

	t.Log("fetching status for job with no conditions and no active pods")
	status, err := exec.Status(context.Background(), executor.ExecutionHandle{ID: "test-job"})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	t.Logf("status: phase=%s", status.Phase)
	if status.Phase != executor.ExecutionPhasePending {
		t.Errorf("phase: got %q, want pending", status.Phase)
	}
}

func TestStatus_Running(t *testing.T) {
	start := metav1.Now()
	client := fake.NewSimpleClientset(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: "test-ns"},
		Status: batchv1.JobStatus{
			Active:    1,
			StartTime: &start,
		},
	})
	exec := kubexec.New(client, "test-ns")

	t.Log("fetching status for job with 1 active pod")
	status, err := exec.Status(context.Background(), executor.ExecutionHandle{ID: "test-job"})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	t.Logf("status: phase=%s startedAt=%v", status.Phase, status.StartedAt)
	if status.Phase != executor.ExecutionPhaseRunning {
		t.Errorf("phase: got %q, want running", status.Phase)
	}
	if status.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
}

func TestStatus_Succeeded(t *testing.T) {
	now := metav1.Now()
	client := fake.NewSimpleClientset(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: "test-ns"},
		Status: batchv1.JobStatus{
			CompletionTime: &now,
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	})
	exec := kubexec.New(client, "test-ns")

	t.Log("fetching status for job with Complete condition")
	status, err := exec.Status(context.Background(), executor.ExecutionHandle{ID: "test-job"})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	t.Logf("status: phase=%s finishedAt=%v", status.Phase, status.FinishedAt)
	if status.Phase != executor.ExecutionPhaseSucceeded {
		t.Errorf("phase: got %q, want succeeded", status.Phase)
	}
	if status.FinishedAt == nil {
		t.Error("expected FinishedAt to be set")
	}
}

func TestStatus_Failed(t *testing.T) {
	client := fake.NewSimpleClientset(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: "test-ns"},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
			},
		},
	})
	exec := kubexec.New(client, "test-ns")

	t.Log("fetching status for job with Failed condition")
	status, err := exec.Status(context.Background(), executor.ExecutionHandle{ID: "test-job"})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	t.Logf("status: phase=%s", status.Phase)
	if status.Phase != executor.ExecutionPhaseFailed {
		t.Errorf("phase: got %q, want failed", status.Phase)
	}
}

func TestStatus_UnknownJob_ReturnsError(t *testing.T) {
	client := fake.NewSimpleClientset()
	exec := kubexec.New(client, "test-ns")

	t.Log("fetching status for non-existent job")
	_, err := exec.Status(context.Background(), executor.ExecutionHandle{ID: "ghost"})
	t.Logf("error: %v", err)
	if err == nil {
		t.Fatal("expected error for unknown job, got nil")
	}
}

func TestCancel_DeletesJob(t *testing.T) {
	client := fake.NewSimpleClientset(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: "test-ns"},
	})
	exec := kubexec.New(client, "test-ns")

	t.Log("canceling job test-job")
	if err := exec.Cancel(context.Background(), executor.ExecutionHandle{ID: "test-job"}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	jobs, _ := client.BatchV1().Jobs("test-ns").List(context.Background(), metav1.ListOptions{})
	t.Logf("jobs remaining after cancel: %d", len(jobs.Items))
	if len(jobs.Items) != 0 {
		t.Errorf("expected job to be deleted, got %d remaining", len(jobs.Items))
	}
}
