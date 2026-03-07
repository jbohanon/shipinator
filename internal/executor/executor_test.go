package executor

import "testing"

func TestExecutionStatus_IsTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status ExecutionStatus
		want   bool
	}{
		{
			name: "pending is non terminal",
			status: ExecutionStatus{
				Phase: ExecutionPhasePending,
			},
			want: false,
		},
		{
			name: "running is non terminal",
			status: ExecutionStatus{
				Phase: ExecutionPhaseRunning,
			},
			want: false,
		},
		{
			name: "succeeded is terminal",
			status: ExecutionStatus{
				Phase: ExecutionPhaseSucceeded,
			},
			want: true,
		},
		{
			name: "failed is terminal",
			status: ExecutionStatus{
				Phase: ExecutionPhaseFailed,
			},
			want: true,
		},
		{
			name: "canceled is terminal",
			status: ExecutionStatus{
				Phase: ExecutionPhaseCanceled,
			},
			want: true,
		},
		{
			name: "unknown phase is non terminal",
			status: ExecutionStatus{
				Phase: ExecutionPhase("unknown"),
			},
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.status.IsTerminal()
			if got != tc.want {
				t.Fatalf("IsTerminal() = %v, want %v", got, tc.want)
			}
		})
	}
}
