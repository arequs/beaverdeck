# BeaverDeck Helm Chart

`BeaverDeck` installs BeaverDeck, a lightweight Kubernetes operations, optimization and triage tool for inspecting cluster state, troubleshooting workloads, and performing common day-2 actions from a single web UI.
Its Cluster Insights workflow helps operations teams find risks before they turn into incidents: it starts with categorized signals for nodes, workloads, GPU, networking, storage, security, and configuration, then lets operators drill into manifests, logs, exec sessions, and remediation actions from the same UI.
Focused Insights sections each load only the data needed for that troubleshooting area.
For GPU-backed clusters, Insights can also highlight visibility gaps and tracking signals (including GPU-related), helping operators confirm where capacity exists and whether the expected monitoring path is available.
It is designed for operators who want fast visibility and common remediation workflows without switching between multiple tools for routine Kubernetes tasks.

## Insights-First Triage

![BeaverDeck Insights](https://raw.githubusercontent.com/arequs/beaverdeck/main/docs/images/insights.png)

BeaverDeck is built around the Insights workflow.
Instead of starting from raw object tables, operators can begin with grouped alerts and health signals, then open the exact resource, logs, or manifest connected to the finding.

Insights are split into focused categories:

- Nodes: readiness, pressure conditions, metrics availability, requested-resource capacity, and underutilization signals
- Workloads: pod and controller health, restart patterns, pending or unhealthy pods, and resource request pressure
- GPU: GPU capacity discovery, allocation pressure, pending GPU workloads, and GPU node workload mix
- Networking: ingress and service signals that help narrow traffic-routing issues
- Storage: PVC binding state, volume usage, storage class visibility, and persistent storage pressure
- Security: pod security context, sensitive literal environment variables, and NetworkPolicy coverage
- Configuration: missing Secret and ConfigMap references

Each category loads only the Kubernetes data needed for that view, so looking at Node Insights does not trigger pod-heavy checks unless that category needs them.
This keeps BeaverDeck responsive on larger clusters and makes the troubleshooting path more predictable.

## Operations Workspace

After an Insight points to a likely issue, BeaverDeck provides the operational tools needed to confirm and act:

- browse cluster objects such as pods, workloads, nodes, services, ingresses, config maps, secrets, PVCs, PVs, storage classes, CRDs and their custom resources, and events
- inspect manifests as YAML and apply edits through the UI
- stream pod and workload logs, including older log history when troubleshooting
- open `exec` sessions into running pods
- run common operational actions such as scale, restart, delete, evict, drain, and uncordon
- review cluster health, warnings, and category-scoped operational insights without jumping between multiple Kubernetes tools
- keep access controlled with users, roles, and namespace-scoped permissions

The CRDs navigation item expands into all definitions visible in the cluster. Namespaced custom resources are queried only in namespaces allowed by the signed-in user's BeaverDeck role. To support arbitrary installed CRDs, the chart ClusterRole grants the BeaverDeck ServiceAccount wildcard get/list/create/update/patch/delete access across API groups and resources; BeaverDeck's application RBAC restricts which users can use that access.

![BeaverDeck Overview](https://raw.githubusercontent.com/arequs/beaverdeck/main/docs/images/overview.png)

## What This Chart Deploys

- `Deployment`
- `Service`
- `ServiceAccount`
- cluster-scoped `ClusterRole` and `ClusterRoleBinding`
- optional `PersistentVolumeClaim`
- optional `Ingress` resource

## Install

Create the target namespace with Helm and install the chart.
BeaverDeck stores auth configuration in a Kubernetes Secret and uses persistence only for non-auth runtime metadata.

```bash
helm upgrade --install beaverdeck oci://ghcr.io/arequs/charts/beaverdeck \
  --version 2.2.4 \
  --namespace beaverdeck \
  --create-namespace \
  --set clusterName=your-cluster-name
```

If you install into another namespace, replace `beaverdeck` in the examples below with that namespace.
Enable chart persistence only if you want non-auth runtime metadata, such as update-check status, to survive pod restarts.

## First Start and Common Configuration

BeaverDeck does not use a pre-created admin password. On first start, open the UI and set the initial admin username and password. BeaverDeck stores the resulting configuration in its Kubernetes Secret.

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

Local users are always available and are used for first initialization.
On first start, BeaverDeck asks for the initial admin username and password in the UI.
After initialization, admins can create local users, reset local passwords, and assign roles.

Roles can be namespace-scoped and can grant different access levels per resource area, including workloads, nodes, exec, apply, insights, users, and roles.

#### Auth Configuration Secret

On startup, BeaverDeck reads auth configuration from a Kubernetes Secret. By default it uses `beaverdeck-config` in the pod namespace with data key `config.yaml`; override with `CONFIG_SECRET_NAME`, `CONFIG_SECRET_NAMESPACE`, and `CONFIG_SECRET_KEY` through `extraEnv` when needed.

If the Secret is missing, BeaverDeck starts the initialization screen and waits for the initial admin username and password in the UI. The Secret is created only after successful initialization. If the Secret exists, BeaverDeck imports it and logs the selected path. If startup import fails, the log includes the failed stage and BeaverDeck exits without overwriting the existing Secret. Fix the Secret content, or delete the Secret to start initialization again.

Admins can export and import the same YAML snapshot from the Admin UI. The snapshot includes local users, roles, Google OAuth config and mappings, and OIDC/Azure Entra ID config and mappings. Local user passwords are exported only as BeaverDeck password hashes, not raw passwords or base64-encoded passwords.

Example Secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: beaverdeck-config
  namespace: beaverdeck
type: Opaque
stringData:
  config.yaml: |
    schema_version: 1
    initialized: true
    roles:
      - name: admin
        mode: admin
      - name: dev
        mode: viewer
        permissions:
          namespaces:
            - apps
          resources:
            clusterroles: view
    users:
      - username: admin
        role: admin
        password_hash: bdk1$180000$<salt-hex>$<digest-hex>
    google:
      config:
        client_id: ""
        client_secret: ""
        hosted_domain: ""
        service_account_json: ""
        delegated_admin_email: ""
      mappings: []
    oidc:
      config:
        provider_name: OpenID Connect
        issuer_url: ""
        client_id: ""
        client_secret: ""
        scopes: openid email profile groups
        hosted_domain: ""
        email_claim: email
        groups_claim: groups
      mappings: []
```

Any role with `mode: "admin"` has full access; its `permissions` value is ignored. Non-admin roles are controlled by their `permissions`. If `permissions` is omitted, the role has no permissions. Resource permission values are compact levels: `view`, `edit`, or `full`.
For Secrets, `view` lists metadata only. Opening a Secret manifest, revealing decoded values, and editing require `secrets: edit`; deleting requires `secrets: full`. User and role administration always requires `mode: admin` and is not delegated through resource permissions.
Valid partial configuration Secrets are completed during startup import. For example, a Secret that contains only a local admin-mode user and one OIDC provider is normalized with the current schema version, default admin role, empty Google config, default OIDC fields, and empty mappings before it is persisted back to the Secret after successful import.

#### Suppressed Insights ConfigMap

Suppressed Insights are global for all users and are stored outside the auth Secret in a ConfigMap. The Helm chart creates it empty by default.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: beaverdeck-suppressed-insights
  namespace: beaverdeck
data:
  suppressed_insights.json: "[]"
```

The value is a JSON array of Insight check keys. Missing or empty means all checks are enabled.
Override `.Values.suppressedInsights.configMapName` or `.Values.suppressedInsights.key` if the default generated name/key is not suitable.

Generate a local user password hash with the BeaverDeck image:

```bash
read -rsp 'Password: ' BDPASS
printf '%s' "$BDPASS" | docker run --rm -i arequs/beaverdeck:1.5.3 hash-password
unset BDPASS
```

Use the printed `bdk1$...` value as `password_hash`. Do not store raw passwords or base64-encoded passwords in the configuration Secret.

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
- `persistence.enabled=true` is optional and only preserves non-auth runtime metadata across pod restarts.

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
| `dataDir` | `/data` | Directory used by BeaverDeck to store non-auth runtime metadata. |
| `extraEnv` | `[]` | Extra environment variables appended to the BeaverDeck container. |
| `fullnameOverride` | `""` | Full override for generated resource names. |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy. |
| `image.repository` | `arequs/beaverdeck` | Container image repository. |
| `image.tag` | `1.5.3` | Container image tag. |
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
| `persistence.enabled` | `false` | Use a PersistentVolumeClaim instead of `emptyDir` for non-auth runtime metadata. |
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
