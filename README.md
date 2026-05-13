[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/beaverdeck)](https://artifacthub.io/packages/search?repo=beaverdeck)

# BeaverDeck

BeaverDeck is a lightweight Kubernetes operations workspace for inspecting cluster state, troubleshooting workloads, and performing common day-2 actions from a single web UI.
Its Cluster Insights workflow helps operations teams find risks before they turn into incidents: it starts with categorized signals for nodes, workloads, networking, and storage, then lets operators drill into manifests, logs, exec sessions, and remediation actions from the same UI.

## Pricing Policy

All functionality currently distributed for free in the official BeaverDeck distribution will remain available for free in official BeaverDeck releases.
Existing free functionality will continue to receive support, updates, and improvements at no charge.
New functionality introduced in future releases may be made available for free or through a paid subscription.
This policy does not change the Apache-2.0 license terms for already distributed source code.

## Quick Start

Install with Helm. Persistent storage is strongly recommended so BeaverDeck keeps users, roles, audit records, auth provider settings, and other stored configuration after pod reschedules or upgrades.

```bash
helm upgrade --install beaverdeck oci://ghcr.io/arequs/charts/beaverdeck \
  --version 2.1.1 \
  --namespace beaverdeck \
  --create-namespace \
  --set persistence.enabled=true \
  --set persistence.size=5Gi \
  --set persistence.storageClass=standard
```

Change `persistence.storageClass` to the storage class available in your cluster.
If you do not expose the app through ingress, port-forward it:

```bash
kubectl -n beaverdeck port-forward svc/beaverdeck 8080:80
```

Then open `http://localhost:8080` and log in with the admin token.
On first start, BeaverDeck writes a bootstrap token to the application log. Enter that token in the UI, then set the admin password.

```bash
kubectl -n beaverdeck logs deployment/beaverdeck
```

See chart details on [Artifact Hub](https://artifacthub.io/packages/search?repo=beaverdeck).

## Insights-First Triage

![BeaverDeck Overview](docs/images/overview.png)

BeaverDeck is built around the Insights workflow. Instead of starting from raw object tables, operators can begin with grouped alerts and health signals, then open the exact resource, logs, or manifest connected to the finding.

Insights are split into focused categories:

- Nodes: readiness, pressure conditions, metrics availability, GPU visibility, and capacity signals
- Workloads: pod and controller health, restart patterns, pending or unhealthy pods, resource request pressure, and security context warnings
- Networking: ingress and service signals that help narrow traffic-routing issues
- Storage: PVC binding state, volume usage, storage class visibility, and persistent storage pressure

Each category loads only the Kubernetes data needed for that view, so looking at Node Insights does not trigger pod-heavy checks unless that category needs them.
This keeps the product fast on larger clusters and makes the troubleshooting path more predictable.

## Operations Workspace

After an Insight points to a likely issue, BeaverDeck provides the operational tools needed to confirm and act:

- browse cluster objects: pods, workloads, nodes, services, ingresses, config maps, secrets, PVCs, PVs, storage classes, CRDs, and events
- inspect manifests as YAML
- edit resources and apply changes through server-side apply
- view pod and workload logs
- open `exec` sessions into running pods
- run common operational actions such as scale, restart, delete, evict, drain, and uncordon
- review cluster health and category-scoped operational insights for nodes, workloads, networking, and storage
- keep actions auditable and access controlled with users, roles, and namespace-scoped permissions

![BeaverDeck Insights](docs/images/insights.png)

## Architecture

- Backend: Go
- Frontend: React + Vite
- Runtime mode: in-cluster only
- Authentication: local users, Google OAuth, generic OpenID Connect, and Azure Entra ID
- Storage: SQLite in `DATA_DIR` for audit and user data

BeaverDeck can run without persistence, but that mode is best treated as temporary or demo-only. For normal use, back `DATA_DIR` with a PVC.

## Authentication Configuration

Configure authentication from the Admin UI after the first local admin user is initialized.

### Local Users and Roles

Local users are always available and are used for first bootstrap.
On first start, BeaverDeck writes a one-time bootstrap token to the pod log; enter that token in the UI and set the admin password.
After initialization, admins can create local users, reset local passwords, revoke sessions, and assign roles.

Roles can be namespace-scoped and can grant different access levels per resource area, including workloads, nodes, exec, apply, audit, insights, users, and roles.

### Google OAuth

Google OAuth supports browser sign-in plus Google Workspace group-to-role mapping.
Configure:

- Google Client ID
- Google Client Secret
- optional Hosted Domain to restrict sign-in to one Workspace domain
- Delegated Admin Email for Google Admin Directory group lookup
- Service Account JSON with domain-wide delegation for group resolution

After saving the provider, configure Google group mappings with group email addresses such as `platform-admins@example.com`.
Mapped groups resolve to BeaverDeck roles during sign-in.

### OpenID Connect

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

### Azure Entra ID

Use an issuer URL in the form `https://login.microsoftonline.com/<tenant-id>/v2.0`.
For Microsoft Graph group lookup, include `User.Read` and `GroupMember.Read.All` in the configured scopes and grant the required Graph consent in Entra ID.
Group mappings can use the group object ID, display name, mail address, or security identifier returned by Microsoft Graph.

## Build Requirements

- Go 1.26.3 or newer
- Node.js 22 or newer for frontend builds

## Repository Layout

- `cmd/server/` - backend entrypoint and embedded web assets
- `internal/api/` - HTTP and WebSocket handlers
- `internal/kube/` - Kubernetes client logic
- `internal/auth/` - auth middleware
- `internal/audit/` - audit log storage
- `internal/users/` - user and role storage
- `ui/` - React application
- `charts/beaverdeck/` - Helm chart

### Notes About RBAC

The chart installs a cluster-scoped RBAC policy because BeaverDeck can inspect and operate on cluster-wide resources such as:

- nodes
- PVs
- storage classes
- CRDs
- namespaces
- node proxy stats for storage usage
- metrics API for node and pod usage

If you want to reduce scope later, do it intentionally: the UI and API currently assume this broader visibility is available.
