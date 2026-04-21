package executor

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type K8sJob struct {
	client    kubernetes.Interface
	namespace string
}

func NewK8sJob(client kubernetes.Interface, namespace string) *K8sJob {
	return &K8sJob{client: client, namespace: namespace}
}

func (e *K8sJob) Execute(ctx context.Context, params map[string]any) (*Result, error) {
	jobSpec, err := e.buildJob(params)
	if err != nil {
		return nil, fmt.Errorf("k8s_job: building manifest: %w", err)
	}

	ns := e.namespace
	if nsOverride, ok := params["namespace"].(string); ok && nsOverride != "" {
		ns = nsOverride
	}

	created, err := e.client.BatchV1().Jobs(ns).Create(ctx, jobSpec, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("k8s_job: creating job: %w", err)
	}

	jobName := created.Name
	err = e.waitForCompletion(ctx, ns, jobName)

	output := map[string]any{
		"job_name":  jobName,
		"namespace": ns,
	}

	if err != nil {
		return &Result{Output: output}, fmt.Errorf("k8s_job: %w", err)
	}

	return &Result{Output: output}, nil
}

func (e *K8sJob) buildJob(params map[string]any) (*batchv1.Job, error) {
	name, _ := params["_job_name"].(string)
	if name == "" {
		return nil, fmt.Errorf("_job_name is required")
	}

	image, _ := params["image"].(string)
	if image == "" {
		return nil, fmt.Errorf("image is required")
	}

	var command []string
	if cmd, ok := params["command"]; ok {
		switch v := cmd.(type) {
		case []any:
			for _, item := range v {
				command = append(command, fmt.Sprintf("%v", item))
			}
		case string:
			command = []string{v}
		}
	}

	var args []string
	if a, ok := params["args"]; ok {
		switch v := a.(type) {
		case []any:
			for _, item := range v {
				args = append(args, fmt.Sprintf("%v", item))
			}
		case string:
			args = []string{v}
		}
	}

	labels, _ := params["_labels"].(map[string]string)
	if labels == nil {
		labels = map[string]string{}
	}

	var backoffLimit int32
	if bl, ok := params["backoff_limit"]; ok {
		switch v := bl.(type) {
		case int:
			backoffLimit = int32(v)
		case float64:
			backoffLimit = int32(v)
		}
	}

	var ttl int32 = 86400
	if t, ok := params["ttl_seconds"]; ok {
		switch v := t.(type) {
		case int:
			ttl = int32(v)
		case float64:
			ttl = int32(v)
		}
	}

	sa, _ := params["service_account"].(string)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: sa,
					Containers: []corev1.Container{
						{
							Name:    "worker",
							Image:   image,
							Command: command,
							Args:    args,
						},
					},
				},
			},
		},
	}

	return job, nil
}

func (e *K8sJob) waitForCompletion(ctx context.Context, namespace, jobName string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			job, err := e.client.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("polling job status: %w", err)
			}
			for _, c := range job.Status.Conditions {
				if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
					return nil
				}
				if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
					return fmt.Errorf("job %s failed: %s", jobName, c.Message)
				}
			}
		}
	}
}
