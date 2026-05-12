# BeaverDeck Helm Chart

`BeaverDeck` installs BeaverDeck, a lightweight Kubernetes operations panel for day-2 cluster work.
From one web UI, BeaverDeck can inspect cluster resources, open manifests, stream and expand logs, run `exec` sessions in pods, and perform common operational actions such as restart, scale, delete, evict, drain, and uncordon.
It also includes an Insights view aimed at fast operational triage: it surfaces warnings and health signals for workloads, nodes, storage, ingress, and resource pressure so operators can spot likely problems before drilling into raw Kubernetes objects and events.
For GPU-backed clusters, Insights can also highlight visibility gaps and tracking signals (including GPU-related), helping operators confirm where capacity exists and whether the expected monitoring path is available.
It is designed for operators who want fast visibility and common remediation workflows without switching between multiple tools for routine Kubernetes tasks.

## What BeaverDeck Helps With

![BeaverDeck Overview](https://raw.githubusercontent.com/arequs/beaverdeck/main/docs/images/overview.png)

From one interface, BeaverDeck can help operators:

- browse cluster objects such as pods, workloads, nodes, services, ingresses, config maps, secrets, PVCs, PVs, storage classes, CRDs, and events
- inspect manifests as YAML and apply edits through the UI
- stream pod and workload logs, including older log history when troubleshooting
- open `exec` sessions into running pods
- run common operational actions such as scale, restart, delete, evict, drain, and uncordon
- review cluster health, warnings, and operational insights without jumping between multiple Kubernetes tools
- keep actions auditable and access controlled with users, roles, and namespace-scoped permissions

![BeaverDeck Insights](https://raw.githubusercontent.com/arequs/beaverdeck/main/docs/images/insights.png)

The Insights section is intended as a first-stop troubleshooting surface.
Instead of starting from raw events or manifests, operators can begin with summarized warnings and health checks across workloads, nodes, storage, ingress, and cluster resource pressure.
On clusters with GPU nodes, Insights can also help validate GPU-related visibility and monitoring coverage so it is easier to confirm where GPU capacity exists and whether exporters and metrics paths are available.

## What This Chart Deploys

- `Deployment`
- `Service`
- `ServiceAccount`
- cluster-scoped `ClusterRole` and `ClusterRoleBinding`
- optional `PersistentVolumeClaim`
- optional `Ingress` resource

## Install

Create the target namespace with Helm and install the chart.
Recommendation: enable persistence from the start to keep your custom configuration safe.

```bash
helm upgrade --install beaverdeck oci://ghcr.io/arequs/charts/beaverdeck \
  --version 2.1.1 \
  --set persistence.enabled=true \
  --set persistence.size=1Gi \
  --set persistence.storageClass=standard \
  --set clusterName=your-cluster-name
```

## First Start and Common Configuration

BeaverDeck does not use a pre-created admin secret.
On first start, the application writes a bootstrap token to the pod log. Open the UI, enter that token, and set the admin password.
Example:

```bash
kubectl -n beaverdeck logs deployment/beaverdeck
```

### Important Notes

- The chart does not create a `Namespace` object. Use `--namespace` and `--create-namespace` during install if needed.
- The RBAC installed by this chart is cluster-scoped because BeaverDeck needs access to cluster-wide resources such as nodes, PVs, CRDs, storage classes, and metrics endpoints.
- `clusterName` is displayed in the UI header. Set it explicitly to a human-readable cluster name.

### Enable Ingress

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
  host: beaverdeck.example.com
  path: /
  pathType: Prefix
  tls:
    - hosts:
        - beaverdeck.example.com
      secretName: beaverdeck-tls
```

## Values

| Value | Default | Description |
| --- | --- | --- |
| `affinity` | `{}` | Pod affinity rules. |
| `allowAllNamespaces` | `true` | If `true`, BeaverDeck can operate across all namespaces allowed by Kubernetes RBAC. |
| `clusterName` | `Cluster name not set` | Cluster name shown in the UI header. |
| `dataDir` | `/data` | Directory used by BeaverDeck to store SQLite data. |
| `extraEnv` | `[]` | Extra environment variables appended to the BeaverDeck container. |
| `fullnameOverride` | `""` | Full override for generated resource names. |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy. |
| `image.repository` | `arequs/beaverdeck` | Container image repository. |
| `image.tag` | `1.4.1` | Container image tag. |
| `ingress.annotations` | `{}` | Ingress annotations. |
| `ingress.className` | `""` | Ingress class name. |
| `ingress.enabled` | `false` | Render a single Ingress resource for BeaverDeck. |
| `ingress.host` | `""` | Ingress host. Leave empty to omit host matching. |
| `ingress.path` | `/` | Ingress path. |
| `ingress.pathType` | `Prefix` | Ingress path type. |
| `ingress.tls` | `[]` | Ingress TLS configuration. |
| `livenessProbe.enabled` | `true` | Enable the liveness probe. |
| `livenessProbe.initialDelaySeconds` | `10` | Initial delay before the liveness probe starts. |
| `livenessProbe.path` | `/healthz` | Liveness probe HTTP path. |
| `livenessProbe.periodSeconds` | `20` | Liveness probe period. |
| `listenAddr` | `:8080` | HTTP listen address passed to the container. |
| `managedNamespace` | `""` | Namespace BeaverDeck treats as its managed namespace. If empty, the pod namespace is used. |
| `nameOverride` | `""` | Override for the chart name portion of generated resource names. |
| `namespaceOverride` | `""` | Override for the namespace used by rendered resources. Defaults to the Helm release namespace. |
| `nodeSelector` | `{}` | Node selector for the pod. |
| `persistence.accessModes` | `["ReadWriteOnce"]` | PVC access modes. |
| `persistence.enabled` | `false` | Use a PersistentVolumeClaim instead of `emptyDir`. Strongly recommended for non-demo installations. |
| `persistence.size` | `1Gi` | PVC size request. |
| `persistence.storageClass` | `default` | StorageClass name for the PVC. Replace with the class available in your cluster. |
| `podAnnotations` | `{}` | Extra pod annotations. |
| `podLabels` | `{}` | Extra pod labels. |
| `rbac.clusterRoleName` | `""` | Override ClusterRole name. If empty, the chart fullname is used. |
| `rbac.create` | `true` | Create ClusterRole and ClusterRoleBinding for BeaverDeck. |
| `readinessProbe.enabled` | `true` | Enable the readiness probe. |
| `readinessProbe.initialDelaySeconds` | `5` | Initial delay before the readiness probe starts. |
| `readinessProbe.path` | `/healthz` | Readiness probe HTTP path. |
| `readinessProbe.periodSeconds` | `10` | Readiness probe period. |
| `resources` | `{}` | Container resource requests and limits. |
| `service.annotations` | `{}` | Extra Service annotations. |
| `service.port` | `80` | Service port. |
| `service.targetPort` | `8080` | Container port exposed by BeaverDeck. |
| `service.type` | `ClusterIP` | Kubernetes Service type. |
| `serviceAccount.annotations` | `{}` | Extra ServiceAccount annotations. |
| `serviceAccount.create` | `true` | Create a dedicated ServiceAccount. |
| `serviceAccount.name` | `""` | Override ServiceAccount name. If empty, the chart fullname is used. |
| `tolerations` | `[]` | Pod tolerations. |
