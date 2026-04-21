# RQ-B1-SEC: Security Threat Model — Controller-as-Job Pattern

**Author:** Raj (Technical Researcher)  
**Date:** 2026-04-21  
**Assignment:** RQ-B1-SEC (Prof. Hayes gap flag, Dr. Chen DM 2306)  
**Parent:** ADR-001 (Bare K8s Client vs Argo Workflows, RQ-B1)  
**Priority:** HIGH  
**Status:** Complete

---

## Executive Summary

**Conclusion: The controller-as-Job pattern introduces a bounded but non-trivial privilege escalation vector.** A compromised controller pod has the ability to create arbitrary Jobs in its namespace, read context data from ConfigMaps/Postgres, and exfiltrate outputs via child Jobs with internet egress. The blast radius is containable with four layered mitigations (namespace-scoped RBAC, NetworkPolicy, PodSecurity Standards, audit logging), but the pattern requires explicit security design — not bolted-on hardening.

**Key finding:** The controller-as-Job has a *larger* blast radius than a compromised step Job, but a *smaller* blast radius than the Argo Workflows controller (which needs permissions across multiple resource types including PVCs, PDBs, secrets, and potentially cluster-wide scope). Our ADR-001 decision to use bare K8s client with namespace-scoped Roles was the correct security call.

**Enterprise adoption gate:** Without the mitigations specified below, this architecture would fail security review at any SOC 2-compliant organization. With them, it's defensible — comparable to Argo Workflows namespaced install or Tekton in single-namespace mode.

---

## 1. Threat Model Scope

### What We're Modeling

The **controller-as-Job** is a K8s `batch/v1 Job` that:
- Parses a chain YAML definition
- Creates child K8s Jobs (one per chain step)
- Monitors child Job completion via Watch API
- Passes context between steps via ConfigMaps or PostgreSQL checkpoint store
- Has a ServiceAccount with `create/get/list/watch/delete` on Jobs, `get/list/watch` on Pods, and `get/list/create/update` on ConfigMaps

### Threat Actor Assumptions

| Actor | Capability | Motivation |
|-------|-----------|------------|
| **External attacker** | RCE in controller pod via dependency vuln or malicious chain YAML | Lateral movement, crypto mining, data exfiltration |
| **Malicious insider** | Craft chain YAML that exploits controller permissions | Privilege escalation, unauthorized resource access |
| **Compromised supply chain** | Poisoned container image for controller or step containers | Persistent backdoor, credential theft |

### Assets at Risk

1. **K8s API access** — controller's ServiceAccount token
2. **Chain context data** — step inputs/outputs in ConfigMaps or Postgres
3. **LLM API credentials** — API keys for Claude/GPT passed to step Jobs
4. **Checkpoint store** — Postgres connection string with chain execution history
5. **Cluster resources** — compute (crypto mining), network (C2/exfil), storage

---

## 2. Attack Surface Analysis

### 2.1 ServiceAccount Token Exposure

**Threat:** By default, K8s mounts the ServiceAccount token at `/var/run/secrets/kubernetes.io/serviceaccount/token`. A compromised controller pod can use this JWT to authenticate to the K8s API server.

**Blast radius with our RBAC (from ADR-001):**

| Permission | What attacker can do | Severity |
|-----------|---------------------|----------|
| `create` Jobs | Spawn arbitrary containers (crypto miners, reverse shells, exfil pods) | **CRITICAL** |
| `delete` Jobs | Kill running chain steps (DoS) | HIGH |
| `get/list/watch` Pods | Enumerate all pods in namespace (reconnaissance) | MEDIUM |
| `create/update` ConfigMaps | Inject malicious context data into downstream steps | **CRITICAL** |
| `get` ConfigMaps | Read all context data from other chain runs in namespace | HIGH |

**Key insight:** The `create Jobs` permission is the most dangerous. An attacker who can create arbitrary Jobs can:
1. Launch a pod with any image (including attacker-controlled images)
2. Mount the same ServiceAccount, creating self-replicating Job chains
3. Use child Job pods to probe the network, access cloud metadata (IMDS at 169.254.169.254), or exfiltrate data via DNS

This maps directly to **Attack Chain B** from ARMO's Kubernetes attack chain taxonomy: compromised pod → RBAC exploitation → privilege escalation via new workloads.

### 2.2 Lateral Movement via Child Jobs

**Threat:** A compromised controller creates child Jobs that probe the network for other services.

**Attack path:**
```
Compromised Controller Pod
    │
    ├─► Create child Job with attacker image
    │       │
    │       ├─► Scan namespace services (K8s DNS: *.svc.cluster.local)
    │       ├─► Query cloud IMDS (169.254.169.254) for IAM credentials
    │       ├─► Connect to Postgres checkpoint store (credentials in env vars)
    │       └─► Probe other namespaces via pod IP (if no NetworkPolicy)
    │
    ├─► Create child Job that mounts additional ConfigMaps/Secrets
    │       └─► Exfiltrate LLM API keys, DB credentials
    │
    └─► Create child Job with hostPath mount (if PodSecurity not enforced)
            └─► Container escape → node access → cluster compromise
```

**Comparison to Argo Workflows:** Argo's controller has the *same* risk profile but with broader blast radius — Argo needs `create/delete` on PVCs, PDBs, and in cluster-install mode, operates across *all* namespaces. Our bare K8s client approach limits this to a single namespace by design.

### 2.3 Data Exfiltration Paths

| Path | Mechanism | Data at Risk | Difficulty |
|------|-----------|-------------|------------|
| **DNS exfil** | Encode data in DNS queries to attacker-controlled domain | ConfigMap contents, SA tokens | Low |
| **HTTP exfil** | Child Job curls data to external endpoint | Chain outputs, LLM responses, API keys | Low |
| **Cloud metadata** | Query IMDS for cloud IAM credentials, pivot to cloud storage | Cloud account access (S3, GCS) | Medium |
| **Postgres exfil** | Read checkpoint store for all chain runs in namespace | Historical chain data, context, outputs | Medium |
| **Log exfil** | Write sensitive data to stdout → shipped to log aggregator | Credentials, PII in chain outputs | Low |

### 2.4 Privilege Escalation Vectors

| Vector | Prerequisite | Impact |
|--------|-------------|--------|
| **Self-replicating Jobs** | `create` on Jobs (we grant this) | Compute exhaustion, persistent access |
| **ConfigMap poisoning** | `create/update` on ConfigMaps (we grant this) | Inject malicious prompts/context into downstream steps |
| **Image substitution** | `create` on Jobs + no admission policy | Run attacker-controlled container images |
| **hostPath mount** | `create` on Jobs + no PodSecurity enforcement | Container escape → node compromise |
| **Secrets enumeration** | Would need `list` on Secrets (we DON'T grant this) | ✅ Mitigated by our RBAC |
| **ClusterRole binding** | Would need `create` on ClusterRoleBindings (we DON'T grant this) | ✅ Mitigated by our RBAC |

### 2.5 Malicious Chain YAML (Insider Threat)

**Threat:** A user crafts a chain YAML that exploits the controller's execution to perform unauthorized actions.

**Examples:**
- Step with `image: attacker/reverse-shell:latest` — controller dutifully creates a Job running the attacker's image
- Step with `env: [{name: EXFIL, value: "$(cat /var/run/secrets/...)"}]` — injects SA token into step environment
- Step with `volumeMounts: [{name: host, mountPath: /host}]` + `volumes: [{name: host, hostPath: {path: /}}]` — attempts host filesystem access

**This is the most likely real-world attack vector.** Unlike external RCE (which requires a vulnerability), malicious YAML only requires a user with `create` permission on chain resources.

---

## 3. Comparative Blast Radius

| Compromised Component | Can Create Workloads? | Namespace Scope | Can Read Secrets? | Can Escape Container? | Overall Risk |
|----------------------|----------------------|-----------------|-------------------|----------------------|-------------|
| **Step Job pod** | ❌ No (no K8s API perms) | N/A | ❌ No | Only if PodSecurity not enforced | LOW |
| **Our Controller Job** | ✅ Yes (Jobs, ConfigMaps) | Single namespace | ❌ No (not in our Role) | Only if PodSecurity not enforced | **MEDIUM-HIGH** |
| **Argo Controller (namespaced)** | ✅ Yes (Pods, PVCs, PDBs, ConfigMaps) | Single namespace | ⚠️ Can list SA tokens | Only if PodSecurity not enforced | HIGH |
| **Argo Controller (cluster)** | ✅ Yes (all above) | **All namespaces** | ⚠️ Yes | Only if PodSecurity not enforced | **CRITICAL** |
| **Tekton Controller** | ✅ Yes (Pods, PipelineRuns) | Configurable | ⚠️ Can access pipeline Secrets | Only if PodSecurity not enforced | HIGH |

**Our position is defensible:** MEDIUM-HIGH is better than both Argo and Tekton in their default configurations. The remaining risk is containable with the mitigations below.

---

## 4. Mitigation Framework (Defense in Depth)

### Layer 1: Bounded ServiceAccount Scope (RBAC)

**Status: Already designed in ADR-001. Validated as correct.**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role  # NOT ClusterRole — namespace-scoped only
metadata:
  name: chain-controller-role
  namespace: agent-chains  # Explicit namespace binding
rules:
# Jobs: create/monitor/cleanup child Jobs
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "get", "list", "watch", "delete"]
# Pods: monitor step execution (read-only)
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]
# ConfigMaps: context passing between steps
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "create", "update", "delete"]
# EXPLICITLY EXCLUDED:
# - secrets (no access to cluster secrets)
# - nodes (no node-level visibility)
# - clusterroles/clusterrolebindings (no privilege escalation)
# - persistentvolumeclaims (no storage provisioning)
# - daemonsets/deployments (no persistent workloads)
```

**Additional hardening:**

```yaml
# Disable auto-mount of SA token on step Jobs (they don't need K8s API access)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: chain-step-runner
  namespace: agent-chains
automountServiceAccountToken: false  # Steps don't talk to K8s API
```

**Recommendation:** Use separate ServiceAccounts for controller vs. step Jobs. The controller SA has the RBAC above; the step SA has `automountServiceAccountToken: false` and zero RBAC permissions. This means even if a step Job is compromised, it has NO K8s API access.

### Layer 2: NetworkPolicy (Lateral Movement Prevention)

```yaml
# Default deny all traffic in the agent-chains namespace
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: agent-chains
spec:
  podSelector: {}
  policyTypes: ["Ingress", "Egress"]
---
# Controller Job: allow K8s API + Postgres only
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: controller-egress
  namespace: agent-chains
spec:
  podSelector:
    matchLabels:
      role: chain-controller
  policyTypes: ["Egress"]
  egress:
  # K8s API server (required for Job creation/monitoring)
  - to:
    - ipBlock:
        cidr: <API_SERVER_IP>/32
    ports:
    - protocol: TCP
      port: 443
  # PostgreSQL checkpoint store
  - to:
    - podSelector:
        matchLabels:
          app: checkpoint-postgres
    ports:
    - protocol: TCP
      port: 5432
  # DNS resolution (required)
  - to:
    - namespaceSelector: {}
      podSelector:
        matchLabels:
          k8s-app: kube-dns
    ports:
    - protocol: UDP
      port: 53
---
# Step Jobs: allow LLM API egress + Postgres only (no K8s API, no internal scanning)
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: step-egress
  namespace: agent-chains
spec:
  podSelector:
    matchLabels:
      role: chain-step
  policyTypes: ["Egress"]
  egress:
  # LLM API endpoints (Claude, OpenAI, etc.)
  - to:
    - ipBlock:
        cidr: 0.0.0.0/0  # Tighten to specific IPs in production
    ports:
    - protocol: TCP
      port: 443
  # Block IMDS (cloud metadata service — prevents IAM credential theft)
  - to:
    - ipBlock:
        cidr: 0.0.0.0/0
        except:
        - 169.254.169.254/32  # AWS/GCP IMDS
        - 169.254.169.253/32  # AWS IMDSv2
  # DNS
  - to:
    - namespaceSelector: {}
      podSelector:
        matchLabels:
          k8s-app: kube-dns
    ports:
    - protocol: UDP
      port: 53
```

**Key controls:**
- **Default deny all:** No pod-to-pod communication unless explicitly allowed
- **Controller can only reach K8s API + Postgres:** Cannot scan other services, cannot reach internet
- **Step Jobs can reach LLM APIs but NOT K8s API:** Even if compromised, no K8s API access
- **IMDS blocked:** Prevents cloud credential theft via metadata service (the #1 lateral movement vector in cloud K8s per Wiz research)
- **No cross-namespace traffic:** Namespace boundary is the hard isolation perimeter

### Layer 3: PodSecurity Standards (Container Escape Prevention)

```yaml
# Enforce restricted PodSecurity on the agent-chains namespace
apiVersion: v1
kind: Namespace
metadata:
  name: agent-chains
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

**What `restricted` enforces on ALL pods in the namespace (including controller and step Jobs):**

| Control | Effect | Threat Mitigated |
|---------|--------|-----------------|
| `runAsNonRoot: true` | No root in containers | Container escape via root exploits |
| `allowPrivilegeEscalation: false` | No setuid/setgid | Privilege escalation within container |
| `capabilities.drop: ["ALL"]` | No Linux capabilities | Kernel exploit surface reduction |
| No `hostPath` volumes | Cannot mount host filesystem | Container escape to node |
| No `hostNetwork` | Cannot use host network stack | Network-level lateral movement |
| No `hostPID/hostIPC` | Cannot see host processes | Process-level lateral movement |
| `seccompProfile: RuntimeDefault` | Syscall filtering | Kernel exploit mitigation |

**Critical for malicious YAML defense:** Even if a user crafts a chain YAML requesting `privileged: true` or `hostPath` mounts, the PodSecurity admission controller will **reject the pod creation at the API server level**. The controller cannot create pods that violate the namespace's security profile, regardless of what the chain YAML specifies.

### Layer 4: Audit Logging & Runtime Detection

```yaml
# K8s Audit Policy — log all Job and ConfigMap mutations in agent-chains
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
# Log all Job creation/deletion at RequestResponse level
- level: RequestResponse
  resources:
  - group: "batch"
    resources: ["jobs"]
  namespaces: ["agent-chains"]
  verbs: ["create", "delete", "patch"]
# Log all ConfigMap mutations (context passing = potential data injection)
- level: RequestResponse
  resources:
  - group: ""
    resources: ["configmaps"]
  namespaces: ["agent-chains"]
  verbs: ["create", "update", "patch", "delete"]
# Log all ServiceAccount token requests
- level: Metadata
  resources:
  - group: ""
    resources: ["serviceaccounts/token"]
  namespaces: ["agent-chains"]
```

**Runtime detection signals (for Falco, Sysdig, or equivalent):**

| Signal | Detection Rule | Indicates |
|--------|---------------|-----------|
| Job created with unknown image | `container.image.repository NOT IN (approved_list)` | Image substitution attack |
| Job created with >1 container | `container.count > 1` on chain step Jobs | Sidecar injection |
| Unexpected egress from controller pod | NetworkPolicy deny log + DNS query to non-approved domain | Data exfiltration attempt |
| SA token read from filesystem | `open /var/run/secrets/kubernetes.io/serviceaccount/token` by non-kubelet process | Credential theft |
| ConfigMap created outside chain run context | ConfigMap without `chain-run-id` label | Context injection attack |
| Rapid Job creation (>N per minute) | Rate exceeds expected chain step count | Self-replicating Job attack / crypto mining |

### Layer 5: Admission Control (Image & Resource Guardrails)

```yaml
# Kyverno policy: restrict images used by chain step Jobs
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: restrict-chain-step-images
spec:
  rules:
  - name: verify-step-image
    match:
      resources:
        kinds: ["Job"]
        namespaces: ["agent-chains"]
        selector:
          matchLabels:
            role: chain-step
    validate:
      message: "Chain step Jobs must use approved images from the internal registry"
      pattern:
        spec:
          template:
            spec:
              containers:
              - image: "registry.internal.company.com/chain-steps/*"
  - name: block-privileged-steps
    match:
      resources:
        kinds: ["Job"]
        namespaces: ["agent-chains"]
    validate:
      message: "Chain Jobs cannot request privileged access"
      deny:
        conditions:
          any:
          - key: "{{ request.object.spec.template.spec.containers[].securityContext.privileged }}"
            operator: Equals
            value: true
  - name: limit-step-resources
    match:
      resources:
        kinds: ["Job"]
        namespaces: ["agent-chains"]
        selector:
          matchLabels:
            role: chain-step
    validate:
      message: "Chain step Jobs are limited to 2 CPU / 4Gi memory"
      pattern:
        spec:
          template:
            spec:
              containers:
              - resources:
                  limits:
                    cpu: "<=2"
                    memory: "<=4Gi"
```

**This addresses the malicious YAML vector:** Even if the controller creates a Job per a malicious chain definition, admission control blocks:
- Unapproved container images (no `attacker/reverse-shell:latest`)
- Privileged containers
- Resource-excessive pods (crypto mining prevention)

---

## 5. Residual Risk Assessment

After all five mitigation layers:

| Threat | Pre-Mitigation | Post-Mitigation | Residual Risk |
|--------|---------------|-----------------|---------------|
| Arbitrary Job creation (crypto mining) | CRITICAL | LOW | Admission control blocks unapproved images; resource limits cap compute |
| SA token theft → K8s API abuse | CRITICAL | LOW | Step Jobs have no SA token; controller scope is namespace-only |
| Lateral movement via network | HIGH | LOW | Default-deny NetworkPolicy; IMDS blocked |
| Container escape → node | HIGH | VERY LOW | PodSecurity `restricted` blocks all escape vectors |
| Data exfiltration via DNS/HTTP | HIGH | MEDIUM | NetworkPolicy limits egress but DNS exfil is hard to fully prevent |
| ConfigMap poisoning (context injection) | HIGH | MEDIUM | Audit logging detects; no prevention without content validation |
| Cloud credential theft via IMDS | CRITICAL | VERY LOW | IMDS IP explicitly blocked in NetworkPolicy |
| Malicious chain YAML | HIGH | LOW | Admission control + PodSecurity + image allowlist |
| Orphan child Jobs on controller crash | MEDIUM | LOW | TTL-after-finished + owner references for GC |

**Remaining MEDIUM risks:**
1. **DNS exfiltration** — mitigated by DNS logging and anomaly detection, but not fully preventable at the network layer. Consider Cilium L7 DNS policies for deeper inspection in high-security environments.
2. **ConfigMap poisoning** — audit logging detects after the fact, but content-level validation of step outputs requires application-layer checks. Recommend: sign step outputs with HMAC using a per-run secret.

---

## 6. Comparison to Production Precedents

### How Argo Workflows Handles This

Argo's controller (per official docs + hardening guide):
- Runs as a Deployment (not a Job) — always-on attack surface
- Default install uses ClusterRole — cross-namespace blast radius
- Namespaced install reduces to single namespace (equivalent to our model)
- Workflow pods run with user-specified ServiceAccount — same separation we propose
- No built-in image allowlisting — relies on external admission controllers
- No built-in NetworkPolicy — relies on cluster-level policies

**Our advantage:** We start with tighter defaults. Argo must be hardened *down* from permissive defaults; our architecture is secure *by design* with the Role specified in ADR-001.

### How Tekton Handles This

Tekton's controller:
- ClusterRole with broad permissions across multiple resource types
- Pipeline Secrets accessible by default (for git credentials, registry auth)
- Relies on namespace isolation and external policy engines

**Our advantage:** We don't need access to Secrets at the controller level. LLM API keys are injected into step Jobs via external secret stores (Vault, External Secrets Operator), not via the controller's RBAC.

---

## 7. Implementation Checklist for Sam

When building the controller-as-Job prototype, apply these security controls:

- [ ] **Separate ServiceAccounts:** `chain-controller-sa` (with RBAC) and `chain-step-sa` (automountServiceAccountToken: false)
- [ ] **Owner references:** Set controller Job as owner of child Jobs for automatic garbage collection
- [ ] **TTL-after-finished:** Set `spec.ttlSecondsAfterFinished: 3600` on child Jobs
- [ ] **Labels on all created resources:** `chain-run-id`, `chain-name`, `step-name`, `role: chain-step`
- [ ] **Image pinning:** Use image digests (sha256), not tags, for step container images
- [ ] **Resource limits:** Set CPU/memory limits on all controller and step Jobs
- [ ] **Non-root execution:** Set `securityContext.runAsNonRoot: true` on all containers
- [ ] **Read-only root FS:** Set `securityContext.readOnlyRootFilesystem: true` where possible
- [ ] **No new privileges:** Set `securityContext.allowPrivilegeEscalation: false`
- [ ] **Credential injection:** Use projected volumes or init containers for LLM API keys, not environment variables

---

## 8. Architecture Decision: Secrets Handling

**Decision: LLM API keys MUST NOT be stored in ConfigMaps or passed via environment variables in Job specs.**

Rationale: ConfigMaps are readable by the controller SA, and Job specs are visible in the K8s API. If the controller is compromised, both are exposed.

**Recommended pattern:**

```
External Secrets Operator / Vault Agent
    │
    ▼
K8s Secret (encrypted at rest)
    │
    ▼ (projected volume mount, read-only)
Step Job Pod
    └── /secrets/llm-api-key (file, mode 0400)
```

The controller Job creates child Jobs that *reference* a Secret by name, but the controller SA does NOT have `get` on Secrets. The Secret is mounted into the step pod by the kubelet, not by the controller. This means:
- Controller compromise does NOT expose LLM API keys
- Step pod compromise exposes only the keys mounted into that specific pod
- Keys rotate via External Secrets Operator without chain YAML changes

---

## Sources

- [Palo Alto Unit 42: Current Threats to K8s Environments](https://unit42.paloaltonetworks.com/modern-kubernetes-threats/)
- [Trend Micro: K8s Threat Modeling Deep Dive](https://www.trendmicro.com/vinfo/us/security/news/security-technology/a-deep-dive-into-kubernetes-threat-modeling)
- [ARMO: 4 K8s Attack Chains and How to Break Them](https://www.armosec.io/blog/kubernetes-attack-chains-and-how-to-break-them/)
- [Wiz: Lateral Movement from K8s to Cloud](https://www.wiz.io/blog/lateral-movement-risks-in-the-cloud-and-how-to-prevent-them-part-2-from-k8s-clust)
- [K8s RBAC Good Practices (official)](https://kubernetes.io/docs/concepts/security/rbac-good-practices/)
- [K8s Pod Security Standards (official)](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [K8s NetworkPolicy (official)](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [Argo Workflows Security Model](https://argo-workflows.readthedocs.io/en/latest/security/)
- [Argo Workflows RBAC](https://argo-workflows.readthedocs.io/en/latest/workflow-rbac/)
- [AquilaX: CI-as-Code Security — Dagger, Tekton, Argo](https://aquilax.ai/blog/ci-as-code-dagger-tekton-security)
- [EVA Security: Argo Workflows Misconfigurations](https://www.evasec.io/blog/argo-workflows-uncovering-the-hidden-misconfigurations)
- [Portainer: K8s RBAC Roles & Best Practices 2026](https://www.portainer.io/blog/kubernetes-rbac)
- [OWASP: K8s Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Kubernetes_Security_Cheat_Sheet.html)
- [Sealos: Practical K8s Security Hardening 2025](https://sealos.io/blog/a-practical-guide-to-kubernetes-security-hardening-your-cluster-in-2025/)
- [Tigera: K8s NetworkPolicy Use Cases 2025](https://www.tigera.io/learn/guides/kubernetes-security/kubernetes-network-policy/)
