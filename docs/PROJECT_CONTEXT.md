# Project Context

## 1.7.0 Cloud AI Analysis

Restart diagnostics can be sent to the separately hosted BeaverDeck Cloud service when `CLOUD_API_URL` is configured. The customer cluster stores only its cloud session in `beaverdeck-cloud-session`; it never receives OpenAI or Stripe secrets.

## Purpose

BeaverDeck is a lightweight Kubernetes operations workspace for inspecting cluster state, triaging and troubleshooting workloads, identifying configuration risks, planning capacity, and performing common day-2 actions from a single web UI.

It focuses on cluster insights, Kubernetes object browsing, manifests, logs, pod exec, capacity signals, and operational actions such as scale, restart, delete, evict, drain, and uncordon. GPU Insights help teams understand scarce GPU capacity, allocation pressure, workload placement, quotas, and metrics availability so they can plan expensive compute more deliberately.

Planned commercial direction, not yet implemented: keep a free tier for clusters with at most five worker nodes and offer paid subscription tiers for larger clusters. The entitlement model, worker-node counting rules, enforcement behavior, and licensing structure are still under evaluation.

## Architecture

- `cmd/server/` contains the Go backend entrypoint and embeds the built frontend assets from `cmd/server/web/dist`.
- `internal/api/` defines HTTP and WebSocket routes for auth, admin configuration, Kubernetes resources, manifests, logs, exec, node/workload/pod actions, and apply operations.
- `internal/auth/` contains auth middleware for protected API routes.
- `internal/config/` reads runtime configuration from environment variables.
- `internal/kube/` wraps Kubernetes access for resources, metrics, insights, manifests, logs, exec, auth config Secret storage, and suppressed Insight ConfigMap storage.
- `internal/users/` manages in-memory auth runtime state, local users, roles, password hashes, OAuth/OIDC config, group mappings, config snapshot import/export, and non-auth SQLite app state.
- `internal/updatecheck/` handles update-check metadata.
- `ui/src/` contains the React application.
- `charts/beaverdeck/` contains the Helm chart.
- Insights are split into Nodes, Workloads, GPU, Networking, Storage, Security, and Configuration sections. There is no separate Capacity Insights section; pod resource checks live under Workloads and node resource checks live under Nodes.
- Dashboard is the standalone first navigation item and the default post-login page. It has no dedicated permission; it conditionally loads and progressively renders health, capacity, readiness, event, inventory, pod-consumer, and Insights summaries from only the resources and selected namespaces the current role may view. Dashboard Insights categories require both `insights: view` and View access to every source-resource family used by that category. Allowed categories are passed as one comma-separated request and evaluated in one `BuildInsights` traversal so shared Kubernetes resources are not rescanned per category; storage checks reuse Nodes already loaded by node/GPU checks. Dashboard events use a server-side Kubernetes `type=Warning` field selector and retain at most 30 recent warnings per namespace. Dashboard PVC/PV inventory calls set `include_metrics=0` to avoid duplicate node `stats/summary` scans, while the dedicated PVC/PV pages retain metrics by default.
- Insight results are grouped in the UI by stable backend `check_type`. Each check type renders as one card with aggregate alert/passing/suppressed counts and a compact list of affected resources; resource navigation, logs, and suppression remain scoped to each underlying result.
- Every current Insight card has an information button that opens the matching public explanation under
  `https://beaverdeck.io/docs/insights-guide/` in a new browser tab. The frontend allowlist in
  `ui/src/lib/insightDocumentation.js` maps all 37 backend `check_type` values to documentation routes.
- `/Users/nail/git_repos/beaverdeck/BeaverDeck-server` is a separate minimal update-check/installation-heartbeat service. It currently has no customer, subscription, plan, entitlement, billing, activation, or signed-license model.

Request flow:

- `cmd/server/main.go` loads config, creates an in-cluster Kubernetes client, opens the user store, imports auth config from a Kubernetes Secret if present, starts the HTTP server, and starts update checks.
- Public routes include auth provider/bootstrap/login endpoints and `/healthz`.
- Protected `/api/*` routes are wrapped with auth middleware.
- Frontend assets are served by the Go server.

Auth/config flow:

- Auth configuration is stored in a Kubernetes Secret, default `beaverdeck-config` with key `config.yaml`.
- If the Secret is absent, the app starts UI initialization and creates the Secret after successful admin initialization.
- If the Secret exists, the app imports it on startup and exits on import failure without overwriting it.
- Admin UI can export/import the same YAML snapshot format.
- Suppressed Insight checks are global and stored in a Kubernetes ConfigMap.
- Login tokens are signed in memory and are not persisted server-side.

## Tech Stack

- Backend: Go `1.27rc3`.
- Go extension modules include `golang.org/x/net` `0.56.0` and `golang.org/x/text` `0.39.0`.
- Frontend: React `19.2.8`, Vite `8.1.5`, lucide-react `1.26.0`, xterm.js with fit addon.
- Kubernetes: `k8s.io/client-go`, `k8s.io/api`, `k8s.io/apimachinery` `0.36.3`.
- Storage: SQLite via `modernc.org/sqlite` `1.56.0` for non-auth runtime metadata.
- Auth/OAuth: local users, Google OAuth, generic OpenID Connect, Azure Entra ID through OIDC, `golang.org/x/oauth2`.
- Config serialization: YAML via `sigs.k8s.io/yaml`.
- Packaging/deploy: Docker multi-stage build, Helm chart.
- Helm metadata uses `https://beaverdeck.io` as the product home page and lists the GitHub repository separately under `sources`.
- Runtime image: distroless static nonroot.

## Runtime & Environments

- Prepared release baseline: BeaverDeck `1.6.1`, Helm chart `2.2.6`, default image tag `1.6.1`.
- Runtime mode is in-cluster only in the application entrypoint; it calls `kube.InCluster()`.
- HTTP listens on `LISTEN_ADDR`, default `:8080`.
- `BASE_PATH` supports non-root ingress paths.
- `DATA_DIR` stores SQLite non-auth runtime metadata, default `/data`.
- The Helm chart deploys a Deployment, Service, ServiceAccount, cluster-scoped RBAC, optional PVC, optional Ingress, and a suppressed Insights ConfigMap.
- CRDs expand in the sidebar into API groups, and each API group expands into the definitions discovered for that group. Clicking a group filters the CRD definition table; clicking a definition lists its resources. Custom resources support manifest, edit, and delete actions, and namespaced instances are queried only from namespaces allowed by BeaverDeck role permissions.
- Applications is a navigation section for application delivery systems. Helm Releases and Argo CD Applications are separate expandable items populated only from selected, role-allowed namespaces; clicking a child opens its revision or deployment history.
- The backend decodes Helm 3 release metadata from Kubernetes Secrets and ConfigMaps, selects the latest revision for the main list, and exposes namespace-checked history and revision detail endpoints. Manage users can open computed values, user-applied values, and rendered resources in a generic read-only YAML bottom-dock tab.
- Helm release decoding caps decompressed data at 32 MiB. An isolated corrupt release record is skipped when other valid releases exist; an error is returned when every stored record is corrupt.
- The backend reads Argo CD `applications.argoproj.io/v1alpha1` custom resources through the dynamic Kubernetes client. It exposes sync and health status, source/destination summaries, deployment history, and Manage-only source configuration plus current created-resource inventory. A missing Argo CD CRD produces an empty list rather than making BeaverDeck fail.
- Restart Diagnostics starts sampling only after the initial Pod informer cache is synchronized, derives a user-facing Pod status from Pod and container state reasons, and stores one latest Secret per exact Pod rather than per container. Snapshots include T-3m/T-1m/T-30s/T-10s jitter-tolerant metrics, all containers' previous logs, node usage/capacity, and PVC/PV metadata and events. Existing workload/container-scoped diagnostic Secrets are consolidated on startup; after every successfully stored incident, Secrets for missing or UID-replaced pods are removed while the current incident is retained.
- Restart Diagnostics keeps its title, workload/container/timestamp subtitle, and close action outside the modal scroll area. Pod and Workload yellow event indicators query only Kubernetes events; Workloads include direct object events and related pod events when the user's permissions allow pod discovery.
- Applications permissions are `none`, `view`, and `edit` (shown as Manage) and cover Helm, Argo CD, and future application sources such as Flux. New roles and legacy roles without the field normalize Applications to None; access must be granted explicitly.
- Apply YAML includes separate sample Widget CRD and Widget custom-resource templates. Apply the CRD first and its instance second so Kubernetes discovery can register the new API before the instance is submitted.
- Apply YAML uses an application-themed, portal-rendered template picker instead of a native HTML select so the open template list follows BeaverDeck colors, spacing, focus, and selected-state styling in both themes.
- The chart ClusterRole grants wildcard get/list/create/update/patch/delete access across API groups and resources so the ServiceAccount can operate on arbitrary installed CRDs. Application RBAC is therefore the user-facing namespace boundary for custom resources.
- Apply YAML rejects any explicit manifest `metadata.namespace` that differs from the selected, role-authorized namespace. Objects without an explicit namespace still use the selected namespace, and cluster-scoped objects remain supported.
- Namespaced aggregate list endpoints use at most eight concurrent Kubernetes requests. API request bodies are capped at 4 MiB; interactive log snapshots are capped at 10,000 lines and 16 MiB of returned data.
- `Follow tail` uses an authenticated SSE stream for both Pod and Workload logs. The backend multiplexes workload-pod
  streams, sends 15-second heartbeats, disables compatible ingress buffering, and truncates an individual line after
  1 MiB while draining its remainder so subsequent lines continue. `Follow tail` controls autoscroll, not collection:
  while the log tab remains active, turning Follow off keeps appending incoming lines to the bounded buffer so no gap
  appears when it is enabled again. The frontend aborts the stream when its tab is hidden, switched, or closed;
  batches UI updates; rejects an SSE frame above 8 MiB; and retains at most 5000 lines or 4 MiB per tab.
- The chart pod uses `RuntimeDefault` seccomp, a read-only root filesystem, and writable `/data` plus `/tmp` volumes. It explicitly grants `create` on `policy/pods/eviction` for pod eviction and node drain workflows.
- Chart persistence defaults to `false`; when disabled, the chart uses `emptyDir` for `/data`.
- `persistence.storageClass` defaults to empty so an enabled PVC uses the cluster's default StorageClass unless an operator selects one explicitly.
- `deploy.sh` is a local helper for Docker buildx, minikube image loading, and Helm upgrade/install. It is not a CI pipeline.
- No `.github`, `.gitlab`, `.gitea`, or `.circleci` workflow files were found in this checkout.

## Configuration

Environment variables read by `internal/config`:

- `LISTEN_ADDR`: HTTP listen address.
- `BASE_PATH`: optional base path for ingress path prefixes.
- `DATA_DIR`: local data directory for non-auth runtime metadata.
- `APP_VERSION`: app version reported by the server/update check.
- `CLUSTER_NAME`: cluster label shown in the UI.
- `POD_NAMESPACE`: current pod namespace.
- `SERVICE_ACCOUNT_NAME`: service account name.
- `MANAGED_NAMESPACE`: namespace treated as managed by default.
- `ALLOW_ALL_NAMESPACES`: whether BeaverDeck may operate across all namespaces allowed by Kubernetes RBAC.
- `CONFIG_SECRET_NAME`: auth configuration Secret name, default `beaverdeck-config`.
- `CONFIG_SECRET_KEY`: auth configuration Secret key, default `config.yaml`.
- `CONFIG_SECRET_NAMESPACE`: auth configuration Secret namespace, default `POD_NAMESPACE`.
- `SUPPRESSED_INSIGHTS_CONFIGMAP_NAME`: ConfigMap name for globally suppressed Insight checks.
- `SUPPRESSED_INSIGHTS_CONFIGMAP_KEY`: ConfigMap key for suppressed Insight JSON array.
- `SUPPRESSED_INSIGHTS_CONFIGMAP_NAMESPACE`: ConfigMap namespace, default `POD_NAMESPACE`.
- `UPDATE_CHECK_URL`: update-check endpoint.
- `UPDATE_CHECK_INTERVAL_HOURS`: update-check interval.
- `UPDATE_CHECK_JITTER_MINUTES`: update-check jitter.
- `RESTART_DIAGNOSTICS_ENABLED`: enable Kubernetes-native restart/eviction incident capture, default `true`.
- `RESTART_DIAGNOSTICS_INTERVAL_SECONDS`: container/node CPU and memory sampling interval, default `10`.
- `RESTART_DIAGNOSTICS_HISTORY_MINUTES`: bounded in-memory history window, minimum/default `5`.
- `RESTART_DIAGNOSTICS_MAX_LOG_LINES`: previous-log line cap per container, default `100`.
- `RESTART_DIAGNOSTICS_MAX_LOG_BYTES_PER_CONTAINER`: previous-log byte cap per container, default `32768`.
- `RESTART_DIAGNOSTICS_MAX_TOTAL_LOG_BYTES`: total previous-log byte cap per incident, default `131072`.
- `RESTART_DIAGNOSTICS_MAX_EVENTS`: relevant event cap per incident, default `20`.

Secret/config notes:

- Do not commit credentials or secret values.
- Auth config YAML includes roles, users with `bdk1$...` password hashes, Google config/mappings, and OIDC config/mappings.
- Role permission YAML uses compact levels such as `clusterroles: view`; non-admin roles with omitted permissions have no permissions.
- `mode: admin` grants full access and does not need explicit permissions.
- Editing an existing manifest requires `edit` for that resource. The separate `apply: edit` permission grants access to arbitrary Apply YAML and is not required for editing an existing object.
- Runtime user, role, Google, generic OIDC, Entra, bootstrap, password, and bulk-import changes are serialized with Secret persistence. Generic OIDC and Azure Entra ID have independent configuration and group mappings in the `oidc` and `entra` snapshot sections and may be enabled simultaneously. Readers do not observe an uncommitted change, and a failed Secret save restores both configuration and exact local session versions.
- Session tokens identify the user and role, while the backend resolves the current role permissions on every authenticated request; role permission updates therefore apply to existing backend sessions.
- `secrets: view` lists Secret metadata only. Secret manifests contain normal Kubernetes base64 `data` values, so opening a manifest, revealing decoded data, and editing all require `secrets: edit`; deletion requires `secrets: full`.
- User and role administration is restricted to `mode: admin`; non-admin resource permissions do not delegate those APIs.
- Suppressed Insight ConfigMap value is a JSON array, default `[]`.

## Database

- SQLite database file is `users.db` under `DATA_DIR`.
- Current SQLite table created by `internal/users/store.go` is `app_state` with key/value metadata.
- Local auth configuration is not kept in SQLite.
- On startup, legacy auth-related tables are dropped if present: `users`, `roles`, `google_config`, `google_group_roles`, `oidc_config`, `oidc_group_roles`, `sessions`, and `external_sessions`.
- `PRAGMA foreign_keys = ON` and `PRAGMA secure_delete = ON` are enabled.
- If auth tables existed, the database is vacuumed after dropping them.

## CI/CD

- No CI/CD workflow directory was found in this checkout.
- `Dockerfile` builds frontend assets with Node 22, builds the Go server with Go 1.27rc3, and copies the binary into a distroless nonroot image.
- Go security baseline is Go `1.27rc3` or newer with `golang.org/x/net` `0.56.0` or newer and `golang.org/x/text` `0.39.0` or newer.
- `ui/package.json` provides `npm run build` and `npm run dev`.
- Go tests are run with `go test ./...`.
- Helm rendering can be checked with `helm template beaverdeck charts/beaverdeck`.
- `deploy.sh` is a local/minikube-oriented helper, not a verified production pipeline.
- The 2026-08-17 maintenance pass applied Kubernetes Go modules `0.36.3`, `modernc.org/sqlite` `1.56.0`, PostCSS `8.5.26`, and nanoid `3.3.18`. Vite remains `8.1.5` and lucide-react remains `1.26.0` because the security fixes did not require feature-version upgrades. Tests, race tests, vet, module verification, UI build, npm audit, Helm lint/render, and `govulncheck` passed.

## Conventions

- Backend code uses Go packages under `internal/`, with feature-oriented files such as `resource_handlers.go`, `mutation_handlers.go`, `operations.go`, and `resources.go`.
- Frontend code uses React components in `ui/src/components`, hooks in `ui/src/hooks`, utility constants/functions in `ui/src/lib`, and global CSS in `ui/src/styles.css`.
- Helm templates use helper templates from `charts/beaverdeck/templates/_helpers.tpl`.
- Keep UI changes consistent with the dense operations-workspace style: compact controls, restrained visual styling, and no landing-page patterns.
- Selectable leaves in nested sidebar trees use the same three-pixel inset so active and hover bounds align with their parent row after the parent's chevron column.
- Use `DelayedTooltip` for compact row-action controls. Its normal pointer delay is `400 ms`, is capped at
  `500 ms`, and keyboard focus shows the tooltip immediately.
- Keep docked log output width-constrained and soft-wrap whitespace and long unbroken tokens rather than introducing
  horizontal scrolling or expanding the dock.
- Resource tables with an existing Delete action share `useBulkSelection` and the compact bulk-selection controls.
  Bulk mutations re-check RBAC per selected object, run sequentially, and retain failed rows selected; Nodes and
  read-only history/event views intentionally have no bulk mode. Workloads additionally support bulk restart when
  every selected item is a Deployment.
- Rebuild `ui/` after frontend changes so embedded assets are current.
- Use `gofmt` for Go changes.
- Prefer project-local patterns and helpers over new abstractions.
- When adding or renaming an Insight `check_type`, update `ui/src/lib/insightDocumentation.js` and publish the matching
  BeaverDeck documentation page in the same change set.

## Known Constraints

- Codex project memory files are local-only and ignored by Git.
- Upgrades from `1.4.1` or earlier do not automatically migrate SQLite auth configuration to the Kubernetes Secret format; operators must prepare the Secret or reinitialize auth before starting `1.5.0`, because startup removes legacy auth tables.
- The app currently assumes in-cluster Kubernetes access at runtime.
- Restart Diagnostics is Kubernetes-native incident reconstruction, not durable monitoring: metric history exists only in the running BeaverDeck process, while only the latest versioned snapshot per pod is persisted as a Kubernetes Secret.
- Restart Diagnostics records Metrics API timestamps separately from collection time and performs a final collection
  as soon as an incident is observed, allowing a delayed Metrics API response that still represents a pre-incident
  point to remain eligible for T-10s and nearby snapshot data.
- Helm RBAC is cluster-scoped because the UI/API inspect and operate on cluster-wide resources such as nodes, PVs, storage classes, CRDs, namespaces, metrics, and node proxy stats.
- `openapi.yaml` remains incomplete: it documents only 11 paths compared with 79 registered routes.
- The measured Go statement coverage baseline is `39.1%`; auth middleware and most HTTP handlers, including mutation routes, have no direct coverage. The frontend has no lint or test command.
- Auth config import failure at startup is fatal by design; the existing Secret must be fixed or deleted for fresh initialization.
- Login sessions are in-memory only; users must sign in again after a pod restart.
- `DATA_DIR` persistence preserves non-auth runtime metadata only, not auth configuration.
- There is currently no license, subscription, entitlement, billing, activation, worker-node limit, or paid-feature gate in BeaverDeck.
- The current Node list path derives display roles from `node-role.kubernetes.io/*` labels, but it also loads all pods and metrics. It is not suitable as a periodic licensing counter without a separate lightweight Node-only path and explicit worker classification.
- BeaverDeck `1.6.1` still sends only `appVersion` to the update-check endpoint. The neighboring BeaverDeck-server
  accepts this legacy request, records a legacy installation identity derived from the source address plus a hash,
  and also supports newer requests with an explicit `installationId`.
- BeaverDeck-server stores installation heartbeats in one local JSON file and uses an optional static bearer token only for its summary endpoint. It has no durable multi-instance database, tenant isolation, billing integration, signed entitlements, activation lifecycle, rate limiting, or key management.
- BeaverDeck-server's Dockerfile currently builds with Go `1.26.3`, below BeaverDeck's established Go `1.26.5` security baseline.
- The public Pricing Policy states that functionality already distributed for free remains free in official releases. Applying a five-worker-node cap to existing functionality would conflict with that statement unless the product policy is deliberately revised.
- Already distributed BeaverDeck source is Apache-2.0 licensed and can be modified and redistributed under that license. Strong tamper-proof enforcement cannot be guaranteed solely inside the open-source in-cluster application; the future commercial/licensing structure requires explicit product and legal review.

## Open Questions

- What production CI/CD system is authoritative for builds, tests, chart publishing, and image publishing?
- Should local `deploy.sh` be retained as-is, updated, or replaced by documented deployment commands?
- Should frontend preferences still use browser `localStorage` for theme and namespace selection, or should they move elsewhere?
- What is the intended release/versioning process for app version, chart version, image tag, changelog, and Artifact Hub annotations?
- What is the desired long-term RBAC scope if users want reduced cluster-wide permissions?
- Does the five-worker-node limit apply to the whole application, only future paid functionality, or only the official supported distribution, given the current Pricing Policy?
- What exactly counts as a worker node: every non-control-plane Node object, only Ready/schedulable nodes, or a peak/rolling count? How should autoscaler bursts, cordoned nodes, NotReady nodes, and temporary overages behave?
- What should happen when a cluster exceeds its tier, a subscription expires, or the licensing service is unavailable: warning, grace period, read-only mode, selective paid-feature disablement, or full denial?
- Will future commercial code use open-core, a separate proprietary edition/module, dual licensing, or paid support/hosted services around the existing Apache-2.0 core?
- Which billing provider, customer/account model, activation limits, cancellation/refund rules, and customer portal are required?
- What stable installation identity and telemetry consent/privacy policy should be used if worker counts and entitlement heartbeats are sent to BeaverDeck-server?
