# Technical Decisions

## 2026-09-09 - Send AI Incidents Through the In-Cluster Backend

The React UI never sends diagnostic data directly to a cloud model provider. The local Go backend reads the existing restart snapshot, emits a bounded redacted DTO, and forwards it only to BeaverDeck Cloud.

## 2026-08-29 — Render The Apply Template Picker In The Application Theme

### Context

The Apply YAML template control used a native HTML select. Its closed control inherited some BeaverDeck styles, but
the open option window was rendered by the browser or operating system and visibly diverged from the application.

### Decision

Use a compact portal-rendered template popover with the same surface, border, typography, hover, focus, and accent
states as BeaverDeck action menus. Keep the existing template names and `loadTemplate` behavior unchanged.

### Rationale

The application cannot consistently theme the popup portion of a native select. A portal also prevents the template
list from being clipped by the fixed toolbar while retaining responsive viewport positioning.

### Consequences

The picker owns outside-click, Escape, resize, and scroll handling and exposes listbox/option accessibility semantics.
Future template-picker styling should continue to reuse the application's existing theme variables.

## 2026-08-29 — Group Insight Results Without Collapsing Resource Actions

### Context

The Insights API intentionally emits one result per checked resource, but rendering every result as a separate card
made repeated findings such as pods running as root difficult to scan.

### Decision

Group visible results in the frontend by the stable backend `check_type`. Render one card per check type with aggregate
severity and counts, then list each affected resource as a compact row. Keep the original result objects and keys so
Open Resource, Open Logs, Ignore, and Restore continue to operate on one resource at a time.

### Rationale

Frontend grouping improves readability without changing the API contract or losing the resource identity required by
existing actions and suppression persistence. `check_type` is a more stable grouping key than user-facing labels.

### Consequences

Summary counters still count individual checks, while the page displays fewer top-level cards. Show all checks may
place passing and alert rows in the same check-type card; the card uses the highest visible severity and reports both
counts.

## 2026-08-15 — Move The Go Security Baseline To Go 1.27rc3

### Context

The container security report identified multiple Go standard-library vulnerabilities in Go `1.26.5`, including
CVE-2026-39821 and CVE-2026-46600. The reported fixed versions include Go `1.27rc3`.

### Decision

Build BeaverDeck with Go `1.27rc3`, declare `go 1.27rc3` in `go.mod`, and pin the container build stage to
`golang:1.27rc3-alpine`.

### Rationale

Using the exact fixed toolchain requested by the security baseline ensures the statically linked Go standard library
in the shipped binary no longer comes from Go `1.26.5`. Pinning the full RC version avoids silently moving between
release candidates.

### Consequences

Go 1.27 is still a release candidate and therefore carries more compatibility risk than a stable patch release.
Full tests, vet, static cross-compilation, container builds, and vulnerability scans should remain part of release
validation. Move to the final Go 1.27 release when it is available and validated.

## 2026-07-24 — Use Controlled Action Tooltips And Bounded Log Output

### Context

Native browser `title` tooltips have browser-controlled timing and can take longer than the operational UI should
require. Action controls can also live inside clipped tables and portal menus. Separately, long unbroken log tokens
such as URLs or serialized payloads could force the bottom dock wider than the viewport.

### Decision

Use the shared portal-based `DelayedTooltip` for pod Logs, Exec, action-menu triggers, and action-menu entries. Show it
after `400 ms` for pointer hover, cap configurable pointer delays at `500 ms`, and show it immediately for keyboard
focus. Keep log containers width-constrained and wrap both normal whitespace and unbroken tokens with
`overflow-wrap: anywhere`.

### Rationale

Application-controlled timing is deterministic, while a body-level portal avoids clipping by table and menu
containers. Soft wrapping keeps all log content readable without allowing one token to resize the operational frame.

### Consequences

New compact row actions should reuse `DelayedTooltip` instead of native `title` text. Log output intentionally has no
horizontal scrolling; exact byte layout remains available in the source stream but is visually wrapped in the dock.

## 2026-07-24 — Keep Secret List Access Metadata-Only

### Context

The role editor described `secrets: view` as List Secrets, but the shared manifest endpoint also accepted that
permission. Kubernetes Secret manifests contain base64-encoded values that are trivially decoded, so list access
effectively exposed the Secret even when the explicit Reveal action remained disabled.

### Decision

Use `secrets: view` only for the metadata list. Require `secrets: edit` for both the normal base64 manifest and decoded
Reveal response, and require `secrets: full` for deletion. Enforce this distinction in the backend and mirror it in
the frontend action checks. Keep user and role administration admin-only and remove its ineffective non-admin rows
from the role editor.

### Rationale

Base64 is an encoding, not a confidentiality boundary. The backend must protect the sensitive response independently
of UI visibility, while the role editor should show only permissions that its APIs actually enforce.

### Consequences

Roles that currently have only `secrets: view` can continue listing Secret name, namespace, type, data-key count, and
age, but can no longer open Secret manifests. Grant `secrets: edit` only to roles allowed to read Secret values.
Existing `users` or `roles` entries in imported permission JSON remain inert, but the UI no longer offers them.

## 2026-07-16 — Raise Go Extension Security Baseline

### Context

Security reports identified CVE-2026-46600 in `golang.org/x/net` `0.55.0` and CVE-2026-56852 in `golang.org/x/text` `0.37.0`.

### Decision

Require `golang.org/x/net` `0.56.0` or newer and `golang.org/x/text` `0.39.0` or newer in the module graph. Keep Go `1.26.5` or newer as the build baseline.

### Rationale

These are the fixed versions reported for the new `x/net` issue and the invalid UTF-8 `norm.Iter` infinite-loop issue in `x/text`.

### Consequences

Dependency updates must preserve these newer minimum versions. Related `golang.org/x/*` modules may move forward when required by module compatibility.

## 2026-06-30 — Map Insight Documentation Through A Frontend Allowlist

### Context

Operators need a direct explanation for every Insight finding. Documentation lives on `beaverdeck.io`, while alert
payloads expose stable backend `check_type` identifiers but no documentation URL.

### Decision

Maintain an explicit frontend mapping from every current `check_type` to its public Insights Guide route. Render a
lucide Info button in the top-right corner of each Insight card with the tooltip `Why is this an issue?`, and open the
fixed mapped URL in a new tab with `noopener,noreferrer`.

### Rationale

An allowlist keeps external navigation independent from alert titles and other runtime text, prevents URL injection,
and handles check names whose public route is not a direct string transformation.

### Consequences

New or renamed Insight checks require a mapping entry and a matching public documentation page. Unknown check types
do not render a documentation button. Insight cards constrain and wrap their action layout so the corner button and
content remain accessible on narrow screens.

## 2026-06-29 — Keep Codex Project Memory Local

### Context

Project memory helps local Codex work but is not part of the public BeaverDeck product or documentation.

### Decision

Ignore `AGENTS.md`, `docs/PROJECT_CONTEXT.md`, `docs/DECISIONS.md`, and `docs/CHANGELOG_AI.md` in the repository.

### Rationale

These files contain local working context and should not be published with the application source.

### Consequences

The files remain available in the local workspace but do not appear as untracked Git content or enter future commits.

## 2026-06-29 — Go Security Baseline

### Context

Security reports identified vulnerabilities fixed in Go `1.26.5` and `golang.org/x/net` `0.55.0`.

### Decision

Use Go `1.26.5` or newer for local and container builds and require `golang.org/x/net` `0.55.0` or newer in the module graph.

### Rationale

These are the first fixed versions for the reported standard-library and `x/net` CVEs. Pinning the module and build image prevents minimum-version selection or an older container builder from reintroducing the affected code.

### Consequences

Dependency updates must preserve these minimum versions. Security validation should include both source and production-like binary `govulncheck` scans.

## 2026-06-22 — Node Underutilization Uses Requested Resources

### Context

Node Insights needed a low-utilization check below 50%, while live node metrics can be unavailable or delayed depending on metrics-server and kubelet scrape state.

### Decision

Report node underutilization when scheduled pod CPU requests and memory requests are both below 50% of node allocatable resources. Keep the check in Node Insights alongside node capacity checks.

### Rationale

Requested resources are available from Kubernetes pod specs and align with the existing capacity planning checks. This avoids false behavior tied to missing or first-scrape live metrics.

### Consequences

The check identifies under-requested or lightly scheduled nodes, not real-time CPU or memory idle percentage. Live metrics remain used where existing pod request-usage checks can access them.

## 2026-06-21 — Reveal Secret Shows Decoded Values

### Context

Kubernetes Secret manifests store `data` values as base64. Operators sometimes need to inspect the decoded values while viewing a Secret manifest, but list-level Secret access should not be enough for decoded-value exposure.

### Decision

Keep Secret manifests in their normal Kubernetes form with base64 `data` values. Add an explicit Reveal Secret action that fetches decoded data values and requires the stronger Secret edit/manage permission.

### Rationale

This matches `kubectl get secret -o yaml` behavior for manifests while making decoded values a deliberate action. The server enforces the reveal permission so the UI button is not the only control.

### Consequences

Secret manifest tabs show base64 data. Users with Secret edit/manage permission can reveal a separate decoded-data panel. Users with only list-level Secret access cannot reveal decoded values.

## 2026-06-17 — Maintain Project Memory Files

### Context

Codex work needs persistent, repository-local context so future tasks start from the same facts, constraints, and recent changes.

### Decision

Create `AGENTS.md`, `docs/PROJECT_CONTEXT.md`, `docs/DECISIONS.md`, and `docs/CHANGELOG_AI.md`.

### Rationale

The repository already has multiple moving parts: Go backend, React frontend, Helm chart, Kubernetes integration, auth configuration, and runtime storage rules. Local memory reduces repeated discovery and lowers the chance of conflicting changes.

### Consequences

Future Codex tasks must read and update these files as part of normal work.

## 2026-06-17 — Auth Configuration Lives In Kubernetes Secret YAML

### Context

BeaverDeck needs auth configuration that can be exported, imported, pre-created, and managed outside the local pod filesystem.

### Decision

Store auth configuration in a Kubernetes Secret, default `beaverdeck-config`, key `config.yaml`. Use YAML snapshots for local users, roles, password hashes, Google OAuth config/mappings, and OIDC/Azure Entra ID config/mappings.

### Rationale

Kubernetes Secret storage matches the in-cluster deployment model and avoids keeping auth config in local SQLite or browser storage.

### Consequences

Startup imports the Secret if it exists. Import failure is fatal and does not overwrite the Secret. If the Secret is missing, the app initializes from an empty runtime config and creates the Secret only after successful initial admin setup.

## 2026-06-17 — Auth Config Is Not Stored In Local SQLite

### Context

The project previously had local SQLite storage paths for auth-like data, but current behavior should keep auth configuration in Kubernetes Secret storage.

### Decision

SQLite is used only for non-auth runtime metadata. Legacy local auth tables are dropped during store initialization.

### Rationale

This keeps auth configuration portable through the Kubernetes Secret and avoids split-brain state between SQLite and Secret data.

### Consequences

`DATA_DIR` persistence does not preserve auth configuration. The auth Secret is the source of truth.

## 2026-06-17 — Suppressed Insights Are Global ConfigMap State

### Context

Suppressed Insight checks should be shared across users and not stored in auth config.

### Decision

Store globally suppressed Insight check keys in a Kubernetes ConfigMap, default key `suppressed_insights.json`.

### Rationale

Suppression state is operational UI state, not auth configuration or secret material.

### Consequences

The Helm chart creates the ConfigMap empty by default. Suppressing and unsuppressing Insight checks updates the ConfigMap.

## 2026-06-17 — Login Tokens Are In-Memory Only

### Context

The app should not persist sessions server-side.

### Decision

Sign login tokens with an in-memory key generated on process start and do not store sessions.

### Rationale

This minimizes persisted auth state and aligns with the current Secret-based auth configuration model.

### Consequences

Users must sign in again after a pod restart.

## 2026-06-17 — Non-Admin Role Permissions Use Compact Levels In Secret YAML

### Context

Role permissions in Secret YAML should be readable and compact.

### Decision

Represent resource permissions as compact levels such as `clusterroles: view`, `workloads: edit`, or `secrets: full`. Omit `permissions` when a non-admin role has no permissions. `mode: admin` grants full access and does not require explicit permission entries.

### Rationale

Compact YAML is easier to review and avoids storing redundant boolean maps.

### Consequences

Admin roles ignore explicit permissions. Missing permissions mean no permissions for non-admin roles.

## 2026-06-17 — Frontend Build Artifacts Are Embedded In The Go Server

### Context

The Go server serves embedded frontend production assets from `cmd/server/web/dist`.

### Decision

Run `npm run build` from `ui/` after frontend source changes.

### Rationale

The Docker build and server embedding expect built assets to be present under the backend asset path.

### Consequences

Frontend changes usually affect both `ui/src/*` and generated files under `cmd/server/web/dist`.

## 2026-06-29 — Separate Existing Manifest Edit From Apply YAML

### Context

Editing an existing Kubernetes object required both the resource-specific `edit` permission and `apply: edit`. This hidden second requirement made a valid role update appear ineffective, including after the user signed in again.

### Decision

Require only the resource-specific `edit` permission to modify an existing object's manifest. Keep `apply: edit` exclusively for the arbitrary Apply YAML workflow.

### Rationale

Resource permissions already describe whether a user may manage that resource. Apply YAML has a broader scope because it can submit arbitrary objects, so it remains an explicit separate privilege.

### Consequences

Granting `workloads: edit`, for example, enables editing existing workload manifests without granting arbitrary Apply YAML access. Backend and frontend enforce the same permission rule.

## 2026-08-15 — Discover Custom Resources Through CRDs And Enforce Namespaces Server-Side

### Context

Operators need to browse instances of every CRD installed in a cluster without adding resource-specific code or RBAC rules for each definition.

### Decision

Resolve each selected CRD to its served storage version and dynamic Kubernetes resource, then list its instances through the dynamic client. For namespaced CRDs, accept only namespaces allowed by the current BeaverDeck role and issue one Kubernetes list request per allowed namespace. Use the existing `crds` permission for manifest, edit, and delete actions. Grant the chart ServiceAccount wildcard get/list/create/update/patch/delete access across API groups and resources.

### Rationale

Kubernetes RBAC cannot anticipate arbitrary CRD API groups and resource names. Per-namespace listing prevents data from inaccessible namespaces from entering the response, instead of fetching cluster-wide and filtering afterward.

### Consequences

The ServiceAccount has broad Kubernetes-level access across API groups and resources, so BeaverDeck's application RBAC is the user-facing authorization boundary. Cluster-scoped custom resources are visible to users with `crds: view`; namespaced resources and their actions remain restricted by the role's namespace allowlist.

The CRD navigation hierarchy is `CRDs → API group → CRD kind`. API groups are independently clickable to show only their CRD definitions and expandable to reveal their kinds.
## 2026-08-16 — Read Helm Releases From Kubernetes Storage

### Context

BeaverDeck runs in-cluster without requiring a Helm CLI binary, but operators need release status and history across their allowed namespaces.

### Decision

Read Helm 3 release records directly from Secrets and ConfigMaps labeled `owner=helm`, decode their base64/gzip JSON metadata, and expose only release name, namespace, revision, status, chart, app version, update time, and description. The main list selects the newest revision per release; history returns all revisions newest-first. Both endpoints enforce the existing namespace allowlist and the section-wide `applications` permission.

### Rationale

This supports Helm's standard in-cluster storage drivers without adding a Helm executable or the full Helm SDK. Restricting the response shape avoids exposing stored values, notes, hooks, or rendered manifests.

### Consequences

Secret and ConfigMap storage are supported; external SQL Helm storage is not discoverable through the Kubernetes API. New roles and legacy roles without an Applications field default to None, preserving least privilege. Manage can read revision values and rendered resources, which may contain sensitive configuration, but does not yet mutate a release. The permission applies to every current and future Applications child rather than being Helm-specific.

Helm lives under the Applications navigation section so future delivery sources such as Argo CD, Flux, Kustomize, or OLM can be added as sibling expandable items.

## 2026-08-16 — Read Argo CD Applications Through Their Kubernetes API

### Context

Operators need Argo CD application status and deployment history alongside Helm releases without requiring the Argo CD CLI, API credentials, or a direct connection to the Argo CD server.

### Decision

Read namespaced `argoproj.io/v1alpha1` Application resources through the existing dynamic Kubernetes client. Query only namespaces selected by and allowed for the signed-in BeaverDeck role. Treat a missing Argo CD Application CRD as an empty inventory. Expose sync and health status plus deployment history to Applications View users; expose historical source configuration and the current revision's created-resource inventory to Applications Manage users in read-only YAML tabs.

### Rationale

The Application custom resource already contains the status, history, source, destination, and managed-resource inventory needed for this first view. Reading it directly keeps deployment simple and reuses BeaverDeck's existing in-cluster client and Applications authorization boundary.

### Consequences

The namespace of the Argo CD Application custom resource is the namespace used for BeaverDeck RBAC queries. Clusters that keep Applications centrally in an Argo CD namespace must grant that namespace to users who should inspect them. The feature is read-only: it does not sync, refresh, rollback, delete, or otherwise mutate Argo CD Applications. Created-resource inventory is available only for the revision matching the current sync revision because Argo CD does not retain that inventory per historical revision.

## 2026-08-17 — Persist Bounded Kubernetes-Native Restart Incident Snapshots

### Context

Operators need evidence from just before a container restart or pod eviction, but BeaverDeck must remain useful without Prometheus, Loki, a database, a PVC, a CRD, or another mandatory service.

### Decision

Maintain a bounded in-memory ring of pod-container, pod, and node CPU/memory samples, preferring Metrics Server and falling back to the kubelet resource endpoint. Detect restart-count transitions and evictions through a shared Pod informer. Process incidents through a bounded queue with fixed workers, collect best-effort previous logs and relevant Kubernetes events only at incident time, then overwrite one JSON version 1 Secret per pod named `beaverdeck-restart-<pod-name>`. Resolve ReplicaSet to Deployment and Job to CronJob only for workload context and owner references, not Secret identity. Consolidate older workload/container-scoped Secrets to the newest snapshot for their recorded pod. After each successful incident write, remove diagnostic Secrets whose recorded pod no longer exists or has a different live UID, but always retain the incident just written so eviction evidence is not deleted immediately. Start accepting informer transitions only after the initial cache sync and first metric sample so historical restart counts do not create misleading cold-start incidents. Capture T-3m, T-1m, T-30s, and T-10s metric points and include node allocatable/capacity and pod PVC/PV metadata and events. Expose summaries and full snapshots through Pods View endpoints with the existing server-side namespace allowlist.

### Rationale

The ring preserves the small pre-incident window that Kubernetes does not retain, while the Secret keeps the latest incident across BeaverDeck restarts using only native APIs. Fixed capacities bound memory, goroutines, queued work, event counts, and log bytes during restart storms. Partial-source failures remain visible without suppressing the rest of the incident record.

### Consequences

This is incident reconstruction rather than time-series monitoring: there are four requested pre-incident points and only the latest snapshot for a pod. Incidents during the initial metric warm-up can still have partial history, and previous logs may already be gone. Secrets can contain application logs and therefore rely on the cluster's Kubernetes Secret access controls. Workload deletion garbage-collects owned snapshots; unmanaged Pod snapshots have no workload owner reference.

## 2026-08-17 — Make Apply Namespace Authorization Document-Aware

### Context

Apply YAML authorized the request's selected namespace, but an individual document could declare a different
`metadata.namespace`. The chart ServiceAccount has broad mutation rights for arbitrary custom resources, so the
request-level check alone was not a complete application RBAC boundary.

### Decision

Decode every submitted YAML/JSON document before applying it and reject any explicit namespace that differs from the
selected, role-authorized namespace. Continue to allow omitted namespaces and cluster-scoped objects.

### Rationale

Authorization must cover the namespace Kubernetes will actually use, not only a parallel request field.

### Consequences

One Apply request cannot intentionally target multiple namespaces. Operators must select and apply each namespace
separately, which keeps the UI and backend authorization model unambiguous.

## 2026-08-17 — Persist Auth Configuration As A Serialized Runtime Transaction

### Context

User, role, bootstrap, Google, and OIDC mutations previously changed memory before updating the Kubernetes Secret.
Failed or reordered writes could leave runtime configuration and durable configuration different.

### Decision

Serialize every runtime auth mutation with its Secret save. Hold readers until persistence succeeds and restore the
previous snapshot plus exact session-version map when it fails. Use the same path for bulk config replacement.

### Rationale

The Kubernetes Secret is the durable source of truth; a successful API response must mean both runtime and durable
state accepted the same version.

### Consequences

Authentication reads briefly wait while a rare configuration write reaches Kubernetes. Persistence errors no longer
expose partial role/password/provider changes or revoke sessions for a password change that did not persist.

## 2026-08-17 — Bound Aggregate Requests And Harden The Default Chart

### Context

Large namespace selections, oversized request bodies and log requests, and gzip-compressed Helm release records could
amplify resource consumption. The chart also assumed a StorageClass literally named `default` and did not opt into
the strongest compatible pod security settings.

### Decision

Limit namespaced list fan-out to eight workers, API bodies to 4 MiB, log tail requests to 10,000 lines and 16 MiB,
SSE log lines to 1 MiB, and decoded Helm releases to 32 MiB. Keep usable Helm inventory when only some stored records
are corrupt. Run the chart with RuntimeDefault seccomp and a read-only root filesystem while mounting writable
`/data` and `/tmp`; leave the StorageClass empty to use the cluster default and grant pod eviction explicitly.

### Rationale

These limits preserve normal operational workflows while preventing a single request or compressed record from
consuming unbounded memory or Kubernetes API concurrency. The chart defaults become portable and defense-in-depth
friendly without removing required write locations.

### Consequences

Very large log output is truncated with an explicit marker, oversized requests fail, and releases above the decode
limit are treated as corrupt. Operators can still override the PVC StorageClass and resource settings through values.

## 2026-08-29 — Isolate Generic OIDC And Azure Entra ID

### Context

The Admin UI presented generic OIDC and Azure Entra ID separately, but both wrote the same backend config and group-mapping slot. Configuring or disabling either provider therefore replaced the other.

### Decision

Persist generic OIDC and Entra as independent `oidc` and `entra` snapshot sections, expose separate admin APIs, mappings, OAuth state cookies, login starts, role resolution, and session auth sources. Keep the historical `/api/auth/oidc/callback` redirect URI for both providers and dispatch callbacks by their isolated state cookie. When importing a legacy snapshot with an Entra-like provider in the shared `oidc` section and no `entra` section, migrate that provider and its mappings to `entra`.

### Rationale

Provider configuration, disable actions, group mappings, and login sessions must not mutate or resolve through another provider. Preserving the callback URI avoids requiring existing Entra app registrations to change during upgrade.

### Consequences

Both sign-in buttons can be enabled at once. Exported snapshots now contain an `entra` section. Existing Entra configuration is migrated on load, while generic OIDC remains in `oidc`.

## 2026-08-29 — Package The Current Worktree As 1.6.1

### Context

BeaverDeck 1.6.0 and Helm chart 2.2.5 are the preceding release baseline. The subsequent worktree changes add the role-scoped Dashboard, group Insights, restyle the Apply YAML template picker, and isolate generic OIDC from Azure Entra ID.

### Decision

Release the complete current Git diff as BeaverDeck `1.6.1` with Helm chart `2.2.6` and default image tag `1.6.1`. Keep the published `1.6.0` changelog unchanged and document only the worktree delta in `docs/changelog/1.6.1.md`.

### Consequences

Application, OpenAPI, frontend package, chart appVersion, chart version, image defaults, examples, Artifact Hub changes, and release documentation advance together. The 1.6.0 release record remains historically accurate.

## 2026-08-29 — Make Dashboard Universal But Its Data Permission-Scoped

### Context

Cluster Health was an admin-only leaf under Insights, which made it unsuitable as the common landing page. Making a dashboard visible to every user must not cause broad aggregate requests or disclose counts from resources the user's role cannot view.

### Decision

Replace Cluster Health with a standalone Dashboard at the top of navigation and make it the initial page after login. Do not add a Dashboard role permission. Instead, conditionally request and render every data source using its existing View permission and the already-authorized namespace selection. Start independent source requests together and update each panel as its response arrives. Request Dashboard Insights only when the role has `insights: view` plus View access to all underlying resource families needed by each category, and combine the allowed category names into one backend traversal instead of rescanning shared Kubernetes objects once per category.

### Rationale

Every role gets a useful, predictable entry point without maintaining another permission that merely duplicates existing resource boundaries. Conditional requests prevent both direct object disclosure and indirect count leakage from inaccessible resource families.

### Consequences

Dashboard content differs by role and may be intentionally sparse. A role with no applicable View permissions still sees the Dashboard shell and an access-scoped empty state. Navigation state is no longer restored across login because each new session starts on Dashboard. A slow Insights scan no longer blocks already-completed health, event, or inventory panels; combined category scans retain the existing single-category API behavior for Insights pages.

## 2026-08-30 — Standardize Bulk Resource Actions And Preserve Metric Event Time

### Context

Only Pods supported multi-selection, long diagnostic log lines could expand the Restart Diagnostics modal, and
Metrics API values were indexed by BeaverDeck collection time. The latter discarded a delayed value observed just
after an incident even when Kubernetes reported that the metric itself was sampled before the incident.

### Decision

Use one frontend selection model for resource tables that already expose Delete, apply the existing resource and
namespace RBAC check to every selected row, execute bulk mutations sequentially, and keep only failures selected.
Exclude Nodes and read-only views; allow Workload bulk restart only for all-Deployment selections. Store Metrics API
timestamps per container, pod, and node and perform a final collection at incident observation time. Constrain the
diagnostic modal and soft-wrap its log payloads. Reconnect a dropped Exec session by replacing only its WebSocket
while preserving the existing tab and terminal history.

### Consequences

Bulk actions have consistent partial-failure behavior across namespaced and cluster-scoped resources without a new
backend batch endpoint. Destructive actions remain subject to the same RBAC level as their single-row equivalent.
Short-lived workloads have a better chance of retaining the last Kubernetes metric without treating genuinely
post-incident samples as history. Exec users can recover interrupted sessions in place.

## 2026-08-31 — Stream Follow Logs With Bounded End-To-End Buffers

### Context

The Logs UI implemented Follow by fetching and replacing a complete bounded log snapshot every 2.5 seconds. Slow
responses could overlap, while the unused Pod SSE reader stopped permanently when `bufio.Scanner` encountered a log
line larger than 1 MiB. Workload logs had no streaming route.

### Decision

Use authenticated fetch-based SSE for Pod and Workload Follow. Multiplex a workload's pod streams on the server,
serialize each event as JSON, emit a heartbeat every 15 seconds, and request compatible reverse proxies not to
buffer the response. Read lines with a bounded reader: retain at most 1 MiB for one line, drain any remainder, mark
the event truncated, and continue with later lines. On the client, cancel inactive streams, reconnect with `tail=0`
when existing content is retained, batch render updates, reject frames above 8 MiB, and retain at most 5000 lines or
4 MiB per tab. Treat `Follow tail` as an autoscroll control while the active log stream continues collecting into the
bounded buffer; explicitly closing, hiding, or switching the tab still cancels its upstream stream.

### Consequences

Follow holds one Kubernetes log stream per selected Pod instead of repeatedly downloading snapshots. A malformed or
very long line cannot terminate the server reader or grow the browser buffer without limit. Workload Follow may hold
one upstream connection for each matching Pod; switching away from the log tab closes those streams. Non-Follow
refresh and older-history loading continue to use bounded snapshots.
