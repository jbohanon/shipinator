package kubernetes

import (
	"context"
	"fmt"

	"git.nonahob.net/jacob/shipinator/internal/executor"
	"github.com/google/uuid"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Executor is an executor.Executor backed by Kubernetes Jobs.
type Executor struct {
	client    kubernetes.Interface
	namespace string
}

// New creates a Kubernetes Executor that submits Jobs into the given namespace.
func New(client kubernetes.Interface, namespace string) *Executor {
	return &Executor{client: client, namespace: namespace}
}

// Submit creates a Kubernetes Job for the given spec and returns a handle
// containing the Job name.
func (e *Executor) Submit(ctx context.Context, spec executor.ExecutionSpec) (executor.ExecutionHandle, error) {
	env := make([]corev1.EnvVar, 0, len(spec.Env))
	for k, v := range spec.Env {
		env = append(env, corev1.EnvVar{Name: k, Value: v})
	}

	var activeDeadlineSeconds *int64
	if spec.Timeout > 0 {
		secs := int64(spec.Timeout.Seconds())
		activeDeadlineSeconds = &secs
	}

	backoffLimit := int32(0)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("shipinator-%s", uuid.New()),
			Namespace: e.namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "main",
							Image:   spec.Image,
							Command: spec.Command,
							Env:     env,
						},
					},
				},
			},
		},
	}

	created, err := e.client.BatchV1().Jobs(e.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return executor.ExecutionHandle{}, fmt.Errorf("create job: %w", err)
	}

	return executor.ExecutionHandle{ID: created.Name}, nil
}

// Status fetches the current state of the Job identified by handle and
// translates it into an ExecutionStatus.
func (e *Executor) Status(ctx context.Context, handle executor.ExecutionHandle) (executor.ExecutionStatus, error) {
	job, err := e.client.BatchV1().Jobs(e.namespace).Get(ctx, handle.ID, metav1.GetOptions{})
	if err != nil {
		return executor.ExecutionStatus{}, fmt.Errorf("get job %q: %w", handle.ID, err)
	}

	return jobToStatus(job), nil
}

// Cancel deletes the Job (and its pods) identified by handle.
func (e *Executor) Cancel(ctx context.Context, handle executor.ExecutionHandle) error {
	policy := metav1.DeletePropagationForeground
	err := e.client.BatchV1().Jobs(e.namespace).Delete(ctx, handle.ID, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
	if err != nil {
		return fmt.Errorf("delete job %q: %w", handle.ID, err)
	}
	return nil
}

func jobToStatus(job *batchv1.Job) executor.ExecutionStatus {
	status := executor.ExecutionStatus{Phase: executor.ExecutionPhasePending}

	if job.Status.StartTime != nil {
		t := job.Status.StartTime.Time
		status.StartedAt = &t
	}

	if job.Status.Active > 0 {
		status.Phase = executor.ExecutionPhaseRunning
		return status
	}

	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
			status.Phase = executor.ExecutionPhaseSucceeded
			if job.Status.CompletionTime != nil {
				t := job.Status.CompletionTime.Time
				status.FinishedAt = &t
			}
			return status
		}
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			status.Phase = executor.ExecutionPhaseFailed
			if job.Status.CompletionTime != nil {
				t := job.Status.CompletionTime.Time
				status.FinishedAt = &t
			}
			return status
		}
	}

	return status
}
