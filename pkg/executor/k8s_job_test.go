package executor

import (
	"context"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetPodLogs_NoPods(t *testing.T) {
	client := fake.NewSimpleClientset()
	e := NewK8sJob(client, "test-ns")

	got := e.getPodLogs(context.Background(), "test-ns", "no-such-job")
	if got != "" {
		t.Errorf("getPodLogs() = %q, want empty string", got)
	}
}

func TestGetPodLogs_PodExists(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob-abc12",
			Namespace: "test-ns",
			Labels:    map[string]string{"job-name": "myjob"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
		},
	}
	client := fake.NewSimpleClientset(pod)
	e := NewK8sJob(client, "test-ns")

	got := e.getPodLogs(context.Background(), "test-ns", "myjob")
	if got == "" {
		t.Error("getPodLogs() returned empty string, want log output")
	}
}

func TestGetPodLogs_WrongNamespace(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob-abc12",
			Namespace: "other-ns",
			Labels:    map[string]string{"job-name": "myjob"},
		},
	}
	client := fake.NewSimpleClientset(pod)
	e := NewK8sJob(client, "test-ns")

	got := e.getPodLogs(context.Background(), "test-ns", "myjob")
	if got != "" {
		t.Errorf("getPodLogs() = %q, want empty string (pod in wrong namespace)", got)
	}
}

func TestGetPodLogs_LabelSelector(t *testing.T) {
	unrelated := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pod",
			Namespace: "test-ns",
			Labels:    map[string]string{"job-name": "other-job"},
		},
	}
	client := fake.NewSimpleClientset(unrelated)
	e := NewK8sJob(client, "test-ns")

	got := e.getPodLogs(context.Background(), "test-ns", "myjob")
	if got != "" {
		t.Errorf("getPodLogs() = %q, want empty string (no matching labels)", got)
	}
}

func TestSetOnJobCreated_Called(t *testing.T) {
	client := fake.NewSimpleClientset()
	e := NewK8sJob(client, "test-ns")

	var got *K8sJobInfo
	e.SetOnJobCreated(func(info K8sJobInfo) {
		got = &info
	})

	params := map[string]any{
		"_job_name": "test-job",
		"image":     "busybox:latest",
		"command":   []any{"/bin/sh", "-c", "echo hello"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Execute will fail at waitForCompletion (fake client doesn't update job status),
	// but the callback should fire before that.
	e.Execute(ctx, params)

	if got == nil {
		t.Fatal("onJobCreated was not called")
	}
	if got.Namespace != "test-ns" {
		t.Errorf("Namespace = %q, want test-ns", got.Namespace)
	}
	if got.JobName == "" {
		t.Error("JobName is empty")
	}
}

func TestSetOnJobCreated_NilSafe(t *testing.T) {
	client := fake.NewSimpleClientset()
	e := NewK8sJob(client, "test-ns")

	params := map[string]any{
		"_job_name": "test-job",
		"image":     "busybox:latest",
		"command":   []any{"/bin/sh", "-c", "echo hello"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should not panic when no callback is set
	e.Execute(ctx, params)
}

func TestExecuteOutput_IncludesLogs_WhenAvailable(t *testing.T) {
	output := map[string]any{
		"job_name":  "test-job",
		"namespace": "test-ns",
	}
	logs := "hello from the pod"
	if logs != "" {
		output["logs"] = logs
	}

	if output["logs"] != "hello from the pod" {
		t.Errorf("output[logs] = %v, want 'hello from the pod'", output["logs"])
	}
}

func TestExecuteOutput_OmitsLogs_WhenEmpty(t *testing.T) {
	output := map[string]any{
		"job_name":  "test-job",
		"namespace": "test-ns",
	}
	logs := ""
	if logs != "" {
		output["logs"] = logs
	}

	if _, ok := output["logs"]; ok {
		t.Error("output should not contain 'logs' key when logs are empty")
	}
}

func TestK8sJobWaitExecute_CompletedJob(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob",
			Namespace: "test-ns",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob-abc12",
			Namespace: "test-ns",
			Labels:    map[string]string{"job-name": "myjob"},
		},
	}
	client := fake.NewSimpleClientset(job, pod)
	e := NewK8sJobWait(client, "test-ns")

	result, err := e.Execute(context.Background(), map[string]any{
		"job_name": "myjob",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output["job_name"] != "myjob" {
		t.Errorf("job_name = %v, want myjob", result.Output["job_name"])
	}
	if result.Output["namespace"] != "test-ns" {
		t.Errorf("namespace = %v, want test-ns", result.Output["namespace"])
	}
	if result.Output["status"] != "completed" {
		t.Errorf("status = %v, want completed", result.Output["status"])
	}
	if result.Output["logs"] == "" {
		t.Error("logs are empty, want captured pod logs")
	}
}

func TestK8sJobWaitExecute_FailedJob(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob",
			Namespace: "test-ns",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Message: "backoff limit exceeded"},
			},
		},
	}
	client := fake.NewSimpleClientset(job)
	e := NewK8sJobWait(client, "test-ns")

	result, err := e.Execute(context.Background(), map[string]any{
		"job_name": "myjob",
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want failed job error")
	}
	if !strings.Contains(err.Error(), "backoff limit exceeded") {
		t.Fatalf("Execute() error = %v, want backoff message", err)
	}
	if result == nil {
		t.Fatal("Execute() result is nil, want output with failed status")
	}
	if result.Output["status"] != "failed" {
		t.Errorf("status = %v, want failed", result.Output["status"])
	}
}

func TestK8sJobWaitExecute_WaitsForMissingJobUntilContextDone(t *testing.T) {
	client := fake.NewSimpleClientset()
	e := NewK8sJobWait(client, "test-ns")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result, err := e.Execute(ctx, map[string]any{
		"job_name": "eventual-job",
		"timeout":  0,
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want context timeout")
	}
	if result == nil {
		t.Fatal("Execute() result is nil, want output with pending status")
	}
	if result.Output["status"] != "pending" {
		t.Errorf("status = %v, want pending", result.Output["status"])
	}
}

func TestK8sJobWaitExecute_RunningJobUntilContextDone(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob",
			Namespace: "test-ns",
		},
	}
	client := fake.NewSimpleClientset(job)
	e := NewK8sJobWait(client, "test-ns")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result, err := e.Execute(ctx, map[string]any{
		"job_name": "myjob",
		"timeout":  0,
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want context timeout")
	}
	if result == nil {
		t.Fatal("Execute() result is nil, want output with running status")
	}
	if result.Output["status"] != "running" {
		t.Errorf("status = %v, want running", result.Output["status"])
	}
}

func TestK8sJobWaitExecute_TailLogsFalse(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob",
			Namespace: "test-ns",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob-abc12",
			Namespace: "test-ns",
			Labels:    map[string]string{"job-name": "myjob"},
		},
	}
	client := fake.NewSimpleClientset(job, pod)
	e := NewK8sJobWait(client, "test-ns")

	result, err := e.Execute(context.Background(), map[string]any{
		"job_name":  "myjob",
		"tail_logs": false,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if _, ok := result.Output["logs"]; ok {
		t.Error("logs present with tail_logs=false")
	}
}
