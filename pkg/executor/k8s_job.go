package executor

import (
	"context"
	"fmt"
	"io"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

	logs := e.getPodLogs(ctx, ns, jobName)

	output := map[string]any{
		"job_name":  jobName,
		"namespace": ns,
	}
	if logs != "" {
		output["logs"] = logs
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

	command := toStringSlice(params["command"])
	args := toStringSlice(params["args"])

	labels, _ := params["_labels"].(map[string]string)
	if labels == nil {
		labels = map[string]string{}
	}

	var backoffLimit int32
	if bl, ok := params["backoff_limit"]; ok {
		backoffLimit = toInt32(bl)
	}

	var ttl int32 = 86400
	if t, ok := params["ttl_seconds"]; ok {
		ttl = toInt32(t)
	}

	sa, _ := params["service_account"].(string)

	volumes, volumeMounts := buildVolumes(params)
	envFrom := buildEnvFrom(params)
	initContainers := buildInitContainers(params, volumes)
	resources := buildResources(params)
	affinity := buildAffinity(params)

	pullPolicy := corev1.PullIfNotPresent
	if pp, ok := params["image_pull_policy"].(string); ok {
		pullPolicy = corev1.PullPolicy(pp)
	}

	container := corev1.Container{
		Name:            "worker",
		Image:           image,
		ImagePullPolicy: pullPolicy,
		Command:         command,
		Args:            args,
		Env:             buildEnvVars(params),
		EnvFrom:         envFrom,
		VolumeMounts:    volumeMounts,
		Resources:       resources,
	}

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
					InitContainers:     initContainers,
					Containers:         []corev1.Container{container},
					Volumes:            volumes,
					Affinity:           affinity,
				},
			},
		},
	}

	return job, nil
}

func buildVolumes(params map[string]any) ([]corev1.Volume, []corev1.VolumeMount) {
	volList, ok := params["volumes"].([]any)
	if !ok {
		return nil, nil
	}

	var volumes []corev1.Volume
	var mounts []corev1.VolumeMount

	for _, item := range volList {
		v, ok := item.(map[string]any)
		if !ok {
			continue
		}

		name, _ := v["name"].(string)
		if name == "" {
			continue
		}

		vol := corev1.Volume{Name: name}

		switch {
		case v["pvc"] != nil:
			pvcName, _ := v["pvc"].(string)
			vol.VolumeSource = corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			}
		case v["config_map"] != nil:
			cmName, _ := v["config_map"].(string)
			vol.VolumeSource = corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			}
		case v["secret"] != nil:
			secretName, _ := v["secret"].(string)
			vol.VolumeSource = corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			}
		default:
			vol.VolumeSource = corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}
		}

		volumes = append(volumes, vol)

		if mountPath, ok := v["mount"].(string); ok && mountPath != "" {
			readOnly, _ := v["read_only"].(bool)
			mounts = append(mounts, corev1.VolumeMount{
				Name:      name,
				MountPath: mountPath,
				ReadOnly:  readOnly,
			})
		}
	}

	return volumes, mounts
}

func buildEnvVars(params map[string]any) []corev1.EnvVar {
	envMap, ok := params["env"].(map[string]any)
	if !ok {
		return nil
	}

	var envVars []corev1.EnvVar
	for k, v := range envMap {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: fmt.Sprintf("%v", v),
		})
	}
	return envVars
}

func buildEnvFrom(params map[string]any) []corev1.EnvFromSource {
	secretList, ok := params["secrets"].([]any)
	if !ok {
		return nil
	}

	var envFrom []corev1.EnvFromSource
	for _, item := range secretList {
		name, ok := item.(string)
		if !ok {
			continue
		}
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
			},
		})
	}
	return envFrom
}

func buildInitContainers(params map[string]any, podVolumes []corev1.Volume) []corev1.Container {
	initList, ok := params["init_containers"].([]any)
	if !ok {
		return nil
	}

	var containers []corev1.Container
	for _, item := range initList {
		ic, ok := item.(map[string]any)
		if !ok {
			continue
		}

		name, _ := ic["name"].(string)
		image, _ := ic["image"].(string)
		if name == "" || image == "" {
			continue
		}

		icPullPolicy := corev1.PullIfNotPresent
		if pp, ok := ic["image_pull_policy"].(string); ok {
			icPullPolicy = corev1.PullPolicy(pp)
		}

		container := corev1.Container{
			Name:            name,
			Image:           image,
			ImagePullPolicy: icPullPolicy,
			Command:         toStringSlice(ic["command"]),
			Args:            toStringSlice(ic["args"]),
			VolumeMounts:    buildContainerVolumeMounts(ic),
		}

		containers = append(containers, container)
	}
	return containers
}

func buildContainerVolumeMounts(containerSpec map[string]any) []corev1.VolumeMount {
	vmList, ok := containerSpec["volume_mounts"].([]any)
	if !ok {
		return nil
	}

	var mounts []corev1.VolumeMount
	for _, item := range vmList {
		vm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := vm["name"].(string)
		mountPath, _ := vm["mount_path"].(string)
		if name == "" || mountPath == "" {
			continue
		}
		mounts = append(mounts, corev1.VolumeMount{
			Name:      name,
			MountPath: mountPath,
		})
	}
	return mounts
}

func buildResources(params map[string]any) corev1.ResourceRequirements {
	res, ok := params["resources"].(map[string]any)
	if !ok {
		return corev1.ResourceRequirements{}
	}

	requirements := corev1.ResourceRequirements{}

	if requests, ok := res["requests"].(map[string]any); ok {
		requirements.Requests = toResourceList(requests)
	}
	if limits, ok := res["limits"].(map[string]any); ok {
		requirements.Limits = toResourceList(limits)
	}

	return requirements
}

func toResourceList(m map[string]any) corev1.ResourceList {
	rl := corev1.ResourceList{}
	for k, v := range m {
		s := fmt.Sprintf("%v", v)
		q, err := resource.ParseQuantity(s)
		if err != nil {
			continue
		}
		rl[corev1.ResourceName(k)] = q
	}
	return rl
}

func buildAffinity(params map[string]any) *corev1.Affinity {
	aff, ok := params["affinity"].(map[string]any)
	if !ok {
		return nil
	}

	podAff, ok := aff["pod_affinity"].(map[string]any)
	if !ok {
		return nil
	}

	required, ok := podAff["required"].(map[string]any)
	if !ok {
		return nil
	}

	topologyKey, _ := required["topology_key"].(string)
	matchLabelsRaw, ok := required["match_labels"].(map[string]any)
	if !ok {
		return nil
	}

	matchLabels := make(map[string]string)
	for k, v := range matchLabelsRaw {
		matchLabels[k] = fmt.Sprintf("%v", v)
	}

	return &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: matchLabels,
					},
					TopologyKey: topologyKey,
				},
			},
		},
	}
}

func toStringSlice(val any) []string {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []any:
		s := make([]string, len(v))
		for i, item := range v {
			s[i] = fmt.Sprintf("%v", item)
		}
		return s
	case string:
		return []string{v}
	}
	return nil
}

func toInt32(val any) int32 {
	switch v := val.(type) {
	case int:
		return int32(v)
	case float64:
		return int32(v)
	}
	return 0
}

const maxLogBytes = 64 * 1024

func (e *K8sJob) getPodLogs(ctx context.Context, namespace, jobName string) string {
	pods, err := e.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || len(pods.Items) == 0 {
		return ""
	}

	var limitBytes int64 = maxLogBytes
	stream, err := e.client.CoreV1().Pods(namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{
		LimitBytes: &limitBytes,
	}).Stream(ctx)
	if err != nil {
		return ""
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return ""
	}
	return string(data)
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
