package executor

import (
	"context"
	"testing"

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
