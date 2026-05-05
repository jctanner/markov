# Running Markov in Kubernetes

This guide covers deploying and running markov as a Kubernetes Job.

## In-Cluster Authentication

When markov runs inside a Kubernetes pod, it automatically detects the in-cluster environment by checking for the ServiceAccount token at `/var/run/secrets/kubernetes.io/serviceaccount/token`. No kubeconfig is needed.

The K8s client resolution order:

1. In-cluster config (ServiceAccount token)
2. `--kubeconfig` flag
3. `KUBECONFIG` environment variable
4. Default kubeconfig loading rules (`~/.kube/config`)

## Namespace Resolution

When running in a pod, the namespace is resolved in this order:

1. Workflow file `namespace` field
2. `--namespace` CLI flag
3. ServiceAccount namespace file (`/var/run/secrets/kubernetes.io/serviceaccount/namespace`)
4. `"default"`

## RBAC Requirements

The ServiceAccount running markov needs these minimum permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: markov-runner
rules:
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["create", "get", "list", "watch", "delete"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get"]
  - apiGroups: [""]
    resources: ["pods/exec"]
    verbs: ["create"]          # Only needed for k8s artifact source
```

## Example: Markov as a K8s Job

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: markov-pipeline
spec:
  ttlSecondsAfterFinished: 3600
  template:
    spec:
      serviceAccountName: markov-runner
      restartPolicy: Never
      containers:
        - name: markov
          image: markov:latest
          command: ["markov", "run", "/config/pipeline.yaml"]
          args:
            - "--verbose"
            - "--callback"
            - "https://hooks.example.com/markov"
          volumeMounts:
            - name: pipeline-config
              mountPath: /config
      volumes:
        - name: pipeline-config
          configMap:
            name: markov-pipeline
```

Mount the workflow YAML as a ConfigMap:

```bash
kubectl create configmap markov-pipeline --from-file=pipeline.yaml
```

## State Store in Pods

By default, markov uses `/tmp/markov-state.db` when running in-cluster. This is ephemeral and lost when the pod terminates.

For durable state (to enable `markov resume` across pod restarts), mount a PersistentVolumeClaim:

```yaml
containers:
  - name: markov
    command: ["markov", "run", "/config/pipeline.yaml",
              "--state-store", "/data/markov-state.db"]
    volumeMounts:
      - name: state
        mountPath: /data
volumes:
  - name: state
    persistentVolumeClaim:
      claimName: markov-state
```

## Callbacks from Pods

Connect markov to external observability systems using callbacks:

```yaml
command: ["markov", "run", "/config/pipeline.yaml",
          "--callback", "https://hooks.example.com/events",
          "--callback-header", "Authorization=Bearer $(TOKEN)",
          "--callback", "jsonl:///data/events.jsonl"]
```

For gRPC callbacks to services within the cluster:

```yaml
command: ["markov", "run", "/config/pipeline.yaml",
          "--callback", "grpc://callback-service.default.svc:50051"]
```

## Container Image

The markov Dockerfile produces a Debian-based image with these tools pre-installed:

- bash, coreutils, curl, git, jq, python3

Build the image:

```bash
docker build -t markov:latest .
```

## What's Next

- [Callbacks Reference](../reference/callbacks.md) — event types and callback configuration
- [State Store Reference](../reference/state-store.md) — checkpoint/resume details
- [CLI Reference](../reference/cli.md) — all commands and flags
