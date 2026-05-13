# BeaverDeck Helm Chart

`BeaverDeck` installs BeaverDeck, a lightweight Kubernetes operations workspace for inspecting cluster state, troubleshooting workloads, and performing common day-2 actions from a single web UI.
Its Cluster Insights workflow helps operations teams find risks before they turn into incidents: it starts with categorized signals for nodes, workloads, networking, and storage, then lets operators drill into manifests, logs, exec sessions, and remediation actions from the same UI.
Nodes, Workloads, Networking, and Storage Insights each load only the data needed for that troubleshooting area.
For GPU-backed clusters, Insights can also highlight visibility gaps and tracking signals (including GPU-related), helping operators confirm where capacity exists and whether the expected monitoring path is available.
It is designed for operators who want fast visibility and common remediation workflows without switching between multiple tools for routine Kubernetes tasks.

## Pricing Policy

All functionality currently distributed for free in the official BeaverDeck distribution will remain available for free in official BeaverDeck releases.
Existing free functionality will continue to receive support, updates, and improvements at no charge.
New functionality introduced in future releases may be made available for free or through a paid subscription.
This policy does not change the Apache-2.0 license terms for already distributed source code.

## Insights-First Triage

![BeaverDeck Overview](https://raw.githubusercontent.com/arequs/beaverdeck/main/docs/images/overview.png)

BeaverDeck is built around the Insights workflow.
Instead of starting from raw object tables, operators can begin with grouped alerts and health signals, then open the exact resource, logs, or manifest connected to the finding.

Insights are split into focused categories:

- Nodes: readiness, pressure conditions, metrics availability, GPU visibility, and capacity signals
- Workloads: pod and controller health, restart patterns, pending or unhealthy pods, resource request pressure, and security context warnings
- Networking: ingress and service signals that help narrow traffic-routing issues
- Storage: PVC binding state, volume usage, storage class visibility, and persistent storage pressure

Each category loads only the Kubernetes data needed for that view, so looking at Node Insights does not trigger pod-heavy checks unless that category needs them.
This keeps BeaverDeck responsive on larger clusters and makes the troubleshooting path more predictable.

## Operations Workspace

After an Insight points to a likely issue, BeaverDeck provides the operational tools needed to confirm and act:

- browse cluster objects such as pods, workloads, nodes, services, ingresses, config maps, secrets, PVCs, PVs, storage classes, CRDs, and events
- inspect manifests as YAML and apply edits through the UI
- stream pod and workload logs, including older log history when troubleshooting
- open `exec` sessions into running pods
- run common operational actions such as scale, restart, delete, evict, drain, and uncordon
- review cluster health, warnings, and category-scoped operational insights without jumping between multiple Kubernetes tools
- keep actions auditable and access controlled with users, roles, and namespace-scoped permissions

![BeaverDeck Insights](https://raw.githubusercontent.com/arequs/beaverdeck/main/docs/images/insights.png)

## What This Chart Deploys

- `Deployment`
- `Service`
- `ServiceAccount`
- cluster-scoped `ClusterRole` and `ClusterRoleBinding`
- optional `PersistentVolumeClaim`
- optional `Ingress` resource

## Install

Create the target namespace with Helm and install the chart.
Recommendation: enable persistence from the start to keep users, roles, audit records, auth provider settings, and custom configuration safe.

```bash
helm upgrade --install beaverdeck oci://ghcr.io/arequs/charts/beaverdeck \
  --version 2.1.1 \
  --namespace beaverdeck \
  --create-namespace \
  --set persistence.enabled=true \
  --set persistence.size=1Gi \
  --set persistence.storageClass=standard \
  --set clusterName=your-cluster-name
```

If you install into another namespace, replace `beaverdeck` in the examples below with that namespace.

## First Start and Common Configuration

BeaverDeck does not use a pre-created admin secret.
On first start, the application writes a bootstrap token to the pod log. Open the UI, enter that token, and set the admin password.
Example:

```bash
kubectl -n beaverdeck logs deployment/beaverdeck
```

If ingress is disabled, use port-forwarding:

```bash
kubectl -n beaverdeck port-forward svc/beaverdeck 8080:80
```

Then open `http://localhost:8080`.

### Authentication

BeaverDeck is initialized with a local admin account and can then be configured from the Admin UI.
Supported sign-in methods:

- local users and roles
- Google OAuth with Google Workspace group-to-role mapping
- generic OpenID Connect with group claim mapping
- Azure Entra ID through OIDC with Microsoft Graph group lookup when Graph scopes are granted

#### Local Users and Roles

Local users are always available and are used for first bootstrap.
On first start, BeaverDeck writes a one-time bootstrap token to the pod log; enter that token in the UI and set the admin password.
After initialization, admins can create local users, reset local passwords, revoke sessions, and assign roles.

Roles can be namespace-scoped and can grant different access levels per resource area, including workloads, nodes, exec, apply, audit, insights, users, and roles.

#### Google OAuth

Google OAuth supports browser sign-in plus Google Workspace group-to-role mapping.
Configure:

- Google Client ID
- Google Client Secret
- optional Hosted Domain to restrict sign-in to one Workspace domain
- Delegated Admin Email for Google Admin Directory group lookup
- Service Account JSON with domain-wide delegation for group resolution

After saving the provider, configure Google group mappings with group email addresses such as `platform-admins@example.com`.
Mapped groups resolve to BeaverDeck roles during sign-in.

#### OpenID Connect

Generic OpenID Connect uses provider discovery and maps roles from the configured groups claim.
Configure:

- Provider Name
- Issuer URL
- Client ID
- Client Secret
- Scopes, usually `openid email profile groups`
- optional Hosted Domain
- Email Claim, usually `email`
- Groups Claim, usually `groups`

Group mappings should match the exact group values returned by the provider in the configured groups claim.

#### Azure Entra ID

For Azure Entra ID, use an issuer URL such as `https://login.microsoftonline.com/<tenant-id>/v2.0`.
For Graph group lookup, include `User.Read` and `GroupMember.Read.All` in scopes and grant the required Microsoft Graph consent.
Group mappings can use the group object ID, display name, mail address, or security identifier returned by Microsoft Graph.

### Important Notes

- The chart does not create a `Namespace` object. Use `--namespace` and `--create-namespace` during install if needed.
- The RBAC installed by this chart is cluster-scoped because BeaverDeck needs access to cluster-wide resources such as nodes, PVs, CRDs, storage classes, and metrics endpoints.
- `clusterName` is displayed in the UI header. Set it explicitly to a human-readable cluster name.
- `persistence.enabled=true` is recommended for any non-demo installation.

### Enable Ingress

```yaml
ingress:
  enabled: true
  className: nginx
  host: example.com
  path: /beaverdeck
  pathType: Prefix
  tls:
    - hosts:
        - example.com
      secretName: example-tls
```

When `ingress.enabled=true`, the chart passes `ingress.path` to BeaverDeck as `BASE_PATH`.
Use a non-root path such as `/beaverdeck` only when the ingress controller forwards that same prefix to the service.

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
