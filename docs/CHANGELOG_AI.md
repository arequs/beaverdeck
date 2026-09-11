# AI Change Log

## 2026-09-09 - Protect In-Cluster Cloud Session Secret

Cloud session Secret metadata is hidden from the Secrets listing and it cannot be read, edited, or deleted through normal manifest/resource APIs. Incident normalizer redaction now explicitly covers AWS credential patterns.

## 2026-09-09 - Prepare 1.7.0 Cloud AI Incident Analysis

Added the in-cluster incident normalization/redaction relay, cloud device-link session handling, restart diagnostic AI action/result UI, optional Helm Cloud endpoint configuration, and release metadata for 1.7.0.

## 2026-08-29 — Add The RBAC-Scoped Dashboard

### Summary

Replaced the admin-only Cluster Health leaf under Insights with a standalone Dashboard at the top of navigation and made it the default post-login page. Added a role-scoped health score, summary cards, capacity/readiness bars, pod status distribution, Insights category chart, top pod consumers, warning events, and resource inventory. Dashboard API calls and panels are gated by existing View permissions; Insights categories also require access to their underlying source resources. Follow-up refinements equalized paired-panel heights, added Pod Health KPI tiles, made data sources render progressively, limited Dashboard events with a server-side Warning selector, combined allowed Insights categories into one backend traversal, reused Nodes across combined node/storage checks, and made Dashboard PVC/PV inventory skip duplicate node volume-stat scans.

### Files changed

- `ui/src/App.jsx`
- `ui/src/components/AppChrome.jsx`
- `ui/src/components/DashboardPage.jsx`
- removed `ui/src/components/ClusterHealthPage.jsx`
- `ui/src/lib/appConstants.js`
- `ui/src/styles.css`
- `internal/api/resource_handlers.go`
- `internal/kube/insights.go`, `internal/kube/insights_test.go`, `internal/kube/operations.go`, `internal/kube/resources_test.go`
- rebuilt `cmd/server/web/dist/`
- `README.md`, `charts/beaverdeck/README.md`, `charts/beaverdeck/Chart.yaml`, `docs/changelog/1.6.1.md`
- `docs/PROJECT_CONTEXT.md`, `docs/DECISIONS.md`, `docs/CHANGELOG_AI.md` (local-only)

### Reason

Operators need an immediate visual overview after login, while non-admin users must see only health and inventory signals backed by resources and namespaces their role can view.

### Validation

- `npm run build` from `ui/`
- headless Chrome screenshots at desktop light, desktop dark, limited-role desktop, and limited-role mobile viewports
- restricted-role request audit: only Pods, Workloads, Events, and permitted Workloads/Security Insights endpoints were requested; Nodes, Networking, and Storage endpoints were not requested
- full-access request audit: one combined Insights request was issued for all seven allowed categories instead of seven category requests
- regression test verifies combined Insights categories list shared Pods, Nodes, and Events only once
- regression test verifies the Dashboard warning-event path sends `type=Warning` as a Kubernetes field selector
- regression test verifies lightweight Dashboard PVC/PV inventory does not list Nodes or collect node volume stats

### Follow-ups

- None.

## 2026-08-31 — Preserve Logs While Follow Autoscroll Is Paused

### Summary

Decoupled the `Follow tail` checkbox from the active SSE connection. Turning Follow off now pauses only automatic
scrolling; incoming Pod or Workload logs continue filling the bounded tab buffer and are immediately available when
Follow is enabled again. Live lines received while an older-history snapshot is loading are retained and merged into
the result instead of being overwritten.

### Files changed

- `ui/src/App.jsx` and `ui/src/components/BottomDock.jsx`
- `docs/changelog/1.6.1.md` and project memory files
- rebuilt embedded frontend assets under `cmd/server/web/dist`

### Reason

Stopping and later recreating the stream with `tail=0` skipped every log entry emitted while Follow was disabled.

### Validation

- `npm run build` from `ui/`
- `go test ./...`
- `go vet ./...`
- `git diff --check`

### Follow-ups

- None.

## 2026-08-29 — Align Apply YAML Template Picker With BeaverDeck

### Summary

Replaced the browser-native Apply YAML template select with a compact portal popover that uses BeaverDeck surfaces,
borders, typography, hover/focus states, and accent selection styling. Added responsive placement, scrolling, outside
click dismissal, Escape dismissal, and listbox accessibility semantics without changing template behavior.

### Files changed

- `ui/src/components/ApplyYamlPage.jsx`
- `ui/src/styles.css`
- rebuilt `cmd/server/web/dist/`
- `docs/PROJECT_CONTEXT.md`, `docs/DECISIONS.md`, `docs/CHANGELOG_AI.md` (local-only)

### Reason

The native browser/OS option window could not follow the application's dark and light theme and looked unrelated to
the rest of BeaverDeck.

### Validation

- `npm run build` from `ui/`
- `go test ./...`
- `git diff --check`
- Browser runtime discovery returned no connected browser sessions, so live visual QA was not available.

### Follow-ups

- Re-run live dark/light theme verification when an in-app or extension browser session is connected.

## 2026-08-29 — Group Insights By Check Type

### Summary

Reworked Insights so repeated findings are presented as one card per stable `check_type` with aggregate severity and
alert/passing/suppressed counts. Each affected Kubernetes resource is now a compact row that retains its own resource,
logs, Ignore, and Restore actions. Removed the obsolete per-result dashboard styling and rebuilt embedded frontend
assets.

### Files changed

- `ui/src/App.jsx`
- `ui/src/components/InsightsPage.jsx`
- `ui/src/styles.css`
- rebuilt `cmd/server/web/dist/`
- `docs/PROJECT_CONTEXT.md`, `docs/DECISIONS.md`, `docs/CHANGELOG_AI.md` (local-only)

### Reason

Repeated alerts such as root-user findings across many pods were rendered as a long sequence of nearly identical
cards and were difficult to scan as one operational issue.

### Validation

- `npm run build` from `ui/`
- `go test ./...`
- `git diff --check`

### Follow-ups

- None.

## 2026-08-17 — Prepare BeaverDeck 1.6.0 And Helm Chart 2.2.5

### Summary

Prepared BeaverDeck `1.6.0` and Helm chart `2.2.5` from the four latest commits plus the complete current working
tree. Added operator-facing release notes for extended CRDs, Applications, Restart Diagnostics, security and
persistence hardening, synchronized every current public version reference, replaced Artifact Hub change metadata,
and expanded chart upgrade warnings for RBAC scope and diagnostic Secret contents.

### Files changed

- `docs/changelog/1.6.0.md`
- `charts/beaverdeck/Chart.yaml`
- `charts/beaverdeck/values.yaml`
- `charts/beaverdeck/README.md`
- `README.md`
- `ui/package.json`, `ui/package-lock.json`
- `openapi.yaml`
- `docs/PROJECT_CONTEXT.md`, `docs/CHANGELOG_AI.md` (local-only)

### Reason

The accumulated CRD, Helm/Argo CD Applications, Restart Diagnostics, Go security, RBAC, durability, dependency, and
resource-bound changes form the `1.6.0` application release and require a synchronized `2.2.5` chart plus explicit
upgrade guidance.

### Validation

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go mod verify`
- `govulncheck ./...` — no vulnerabilities
- `npm audit` — no vulnerabilities
- `npm run build` passed as `beaverdeck-ui@1.6.0`
- JSON validation passed for `ui/package.json`, `ui/package-lock.json`, and `charts/beaverdeck/values.schema.json`
- `helm lint --strict charts/beaverdeck`
- `helm template` rendered chart `2.2.5`, app/image/`APP_VERSION` `1.6.0`, wildcard custom-resource RBAC, pod eviction, seccomp, and read-only rootfs
- `helm package` created `/private/tmp/beaverdeck-2.2.5.tgz`; packaged metadata and README were verified
- `git diff --check`

### Follow-ups

- Publish the `1.6.0` image and `2.2.5` chart after review.

## 2026-08-17 — Full Codebase Audit

### Summary

Performed a read-only audit of the Go backend, React frontend, Kubernetes integrations, authentication and RBAC boundaries, Helm chart, API contract, dependencies, and test coverage. No application code or dependency files were changed. The audit identified a namespace-authorization bypass in Apply YAML, non-atomic auth-config persistence, stale OpenAPI documentation, unbounded namespace request fan-out, several transport and deployment hardening gaps, and concentrated test/refactoring debt.

### Files changed

- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Dependency validation

- The current npm lockfile reports one high and one moderate transitive build-time vulnerability through Vite (`nanoid` `3.3.16` and `postcss` `8.5.22`).
- A minimal lockfile-only `npm audit fix` retained Vite `8.1.5` and lucide-react `1.26.0`, updated only `postcss` to `8.5.26` and `nanoid` to `3.3.18`, built successfully, and left zero audit findings.
- An isolated Vite `8.2.1` plus lucide-react `1.31.0` update also built successfully and resolved the same transitive findings.
- Kubernetes module updates from `0.36.2` to `0.36.3`, the current `k8s.io/utils` update, and `modernc.org/sqlite` `1.56.0` passed module verification, vet, tests, and `govulncheck` both independently and as a combined update set.
- Go `1.27rc3` remains pinned because the final Go `1.27` release is not yet available as of the audit date.

### Validation

- `go mod verify`
- `go vet ./...`
- `go test ./...`
- `go test -race ./...`
- `go test -shuffle=on -count=5 ./...`
- `go test -coverprofile=/private/tmp/beaverdeck-audit-cover.out ./...` (`39.1%` statements)
- source and static Linux binary `govulncheck` (no findings)
- current UI production build
- `npm audit --json`
- isolated upgraded UI production build and `npm audit --json`
- `helm lint --strict charts/beaverdeck`
- `helm template beaverdeck charts/beaverdeck --debug`

### Follow-ups

- Fix the Apply YAML namespace boundary first and add an HTTP regression test.
- Make config mutations persistence-safe and ordered.
- Apply the independently validated dependency updates, then add frontend lint/tests and API contract validation to CI.

## 2026-08-17 — Refine Restart Snapshot Retention And Event UX

### Summary

Changed the oldest Restart Diagnostics metric point from T-5m to T-3m, moved the modal title/subtitle/close action outside its scroll area, added post-incident cleanup for snapshot Secrets belonging to missing or UID-replaced pods, and removed the Workloads warning indicator's log fallback in favor of direct workload and related-pod events.

### Files changed

- `internal/kube/restart_diagnostics.go`
- `internal/kube/restart_diagnostics_test.go`
- `ui/src/App.jsx`
- `ui/src/components/RestartDiagnosticModal.jsx`
- `ui/src/components/WorkloadsPage.jsx`
- `ui/src/styles.css`
- rebuilt `cmd/server/web/dist/`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/DECISIONS.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

The modal header disappeared during long-snapshot scrolling, stale snapshots accumulated after pods disappeared, and the Workloads event affordance could unexpectedly display logs. The requested incident window now starts at three minutes.

### Validation

- Added regression coverage for the exact metric offsets and stale/mismatched-UID Secret cleanup while retaining current, live, and malformed entries.
- `GOCACHE=/private/tmp/beaverdeck-go-cache go test ./...`
- `GOCACHE=/private/tmp/beaverdeck-go-cache go vet ./...`
- `npm run build` from `ui/`
- `helm lint --strict charts/beaverdeck`
- `helm template beaverdeck charts/beaverdeck --namespace beaverdeck`
- `git diff --check`
- Browser visual validation was unavailable because the connected browser runtime exposed no browser instance.

### Follow-ups

- None.

## 2026-08-17 — Make Restart Diagnostics Pod-Scoped And Storage-Aware

### Summary

Corrected the Pods status and diagnostic indicators, consolidated Restart Diagnostics to one latest Secret and one UI entry per exact pod, and made the event warning strictly event-only. Diagnostic snapshots now tolerate sampling jitter, skip misleading cold-start incidents before metric history begins, report node usage with allocatable/capacity totals, and include pod PVC/PV metadata plus storage events.

### Files changed

- `internal/kube/client_base.go`
- `internal/kube/resources.go`
- `internal/kube/resources_test.go`
- `internal/kube/restart_diagnostics.go`
- `internal/kube/restart_diagnostics_test.go`
- `ui/src/App.jsx`
- `ui/src/components/PodsPage.jsx`
- `ui/src/components/RestartDiagnosticModal.jsx`
- `ui/src/styles.css`
- `README.md`
- `charts/beaverdeck/README.md`
- rebuilt `cmd/server/web/dist/`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/DECISIONS.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

Container-scoped identities produced duplicate controls and Secrets for multi-container pods, direct use of Pod phase hid actionable container failure reasons such as OOMKilled, informer cold-start events lacked metric history, and snapshots did not preserve enough node or persistent-storage context for incident analysis.

### Validation

- Backend regression tests cover OOMKilled/CrashLoopBackOff/terminating statuses, one incident per pod, jitter-tolerant history lookup, deterministic hash-free Secret naming and legacy consolidation, node totals, all-container logs, and PVC/PV event capture.
- `GOCACHE=/private/tmp/beaverdeck-go-cache go test ./...`
- `GOCACHE=/private/tmp/beaverdeck-go-cache go vet ./...`
- `npm run build` from `ui/`
- `helm lint --strict charts/beaverdeck`
- `helm template beaverdeck charts/beaverdeck --namespace beaverdeck`
- `git diff --check`
- Browser visual validation was unavailable because the connected browser runtime exposed no browser instance.

### Follow-ups

- None.

## 2026-08-15 — Upgrade Go Toolchain To 1.27rc3

### Summary

Raised the module and container build toolchain from Go `1.26.5` to Go `1.27rc3` to address the reported standard
library vulnerabilities in binaries built with Go `1.26.5`.

### Files changed

- `go.mod`
- `Dockerfile`
- `README.md`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/DECISIONS.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

The security report identified CVE-2026-39821, CVE-2026-46600, CVE-2026-33818, CVE-2026-56853,
CVE-2026-56858, CVE-2026-56859, CVE-2026-56860, and CVE-2026-56862 in Go stdlib `1.26.5`, with
Go `1.27rc3` listed as fixed.

### Validation

- `GOPATH=/private/tmp/beaverdeck-go127rc3-gopath GOCACHE=/private/tmp/beaverdeck-go127rc3-cache go test ./...` passed with Go `1.27rc3`.
- `go vet ./...` passed with Go `1.27rc3`.
- `go mod verify` reported `all modules verified`.
- A stripped static Linux amd64 server binary built successfully with Go `1.27rc3`.
- Binary-mode `govulncheck` reported `No vulnerabilities found` for the Go `1.27rc3` static binary.
- `docker build --pull` completed successfully with `golang:1.27rc3-alpine`, including the frontend build,
  Go arm64 build, and distroless runtime image assembly.

### Follow-ups

- Replace the RC toolchain with the final Go 1.27 release after it becomes available and passes the same validation.

## 2026-07-24 — Prepare BeaverDeck 1.5.3 And Helm Chart 2.2.4

### Summary

Prepared BeaverDeck `1.5.3` and Helm chart `2.2.4`. Added deterministic `400 ms` portal tooltips to pod Logs, Exec,
and shared action menus, constrained docked logs so long tokens wrap, included the Secret permission hardening, and
updated current frontend dependencies within their existing major versions.

### Files changed

- `internal/api/resource_handlers.go`
- `internal/api/resource_handlers_test.go`
- `ui/src/components/ActionMenu.jsx`
- `ui/src/components/DelayedTooltip.jsx`
- `ui/src/components/PodsPage.jsx`
- `ui/src/components/ResourcePages.jsx`
- `ui/src/lib/appConstants.js`
- `ui/src/styles.css`
- `ui/package.json`
- `ui/package-lock.json`
- rebuilt `cmd/server/web/dist/`
- `charts/beaverdeck/Chart.yaml`
- `charts/beaverdeck/values.yaml`
- `charts/beaverdeck/README.md`
- `README.md`
- `docs/changelog/1.5.3.md`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/DECISIONS.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

Operational action labels need predictable hover timing, and diagnostic output must remain readable without changing
the dock width. The accumulated UI and permission fixes also needed a synchronized application and chart release.

### Validation

- `npm run build` passed for `beaverdeck-ui@1.5.3` with Vite `8.1.5`.
- `npm audit --json` reported zero vulnerabilities; `npm outdated --json` returned no updates.
- `go test ./...`, `go vet ./...`, and `go mod verify` passed.
- A production-like stripped static Go binary built successfully.
- Source and binary `govulncheck` scans reported no vulnerabilities.
- `helm lint --strict charts/beaverdeck` passed.
- `helm template` rendered chart `2.2.4`, app version `1.5.3`, image `arequs/beaverdeck:1.5.3`, and
  `APP_VERSION=1.5.3`.
- `helm package` created `/private/tmp/beaverdeck-2.2.4.tgz`; packaged chart metadata and README were verified.
- Live hover timing and visual wrapping could not be exercised because the connected browser runtime exposed no
  browser instance; source timing, CSS constraints, and the production frontend build were verified.
- `git diff --check` passed.

### Follow-ups

- Publish the `1.5.3` image and `2.2.4` chart after review.

## 2026-07-24 — Separate Secret Listing From Secret Data Access

### Summary

Changed `secrets: view` to metadata-only list access. Secret manifests and decoded Reveal now require
`secrets: edit` in both the backend and UI. Audited the role matrix against every protected API action, removed
ineffective non-admin `users` and `roles` rows, and clarified pod eviction and exec prerequisites.

### Files changed

- `internal/api/resource_handlers.go`
- `internal/api/resource_handlers_test.go`
- `ui/src/components/ResourcePages.jsx`
- `ui/src/lib/appConstants.js`
- rebuilt `cmd/server/web/dist/`
- `README.md`
- `charts/beaverdeck/README.md`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/DECISIONS.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

A Kubernetes Secret manifest exposes the complete value set as reversible base64. The List Secrets permission
therefore must not authorize the shared manifest endpoint, and the role editor must match backend enforcement.

### Validation

- Added a backend regression test proving a `secrets: view` role receives `403` for both normal and decoded Secret
  manifest requests.
- Added a table-driven test covering manifest permission requirements for every supported resource group.
- `go test ./...` passed.
- `npm run build` passed and regenerated embedded frontend assets.
- `git diff --check` passed.

### Follow-ups

- None.

## 2026-08-17 — Add Restart Diagnostics Incident Snapshots

### Summary

Added a Kubernetes-native Restart Diagnostics workflow. BeaverDeck now samples bounded container/node metrics, detects restart and eviction transitions with a Pod informer, and persists the latest versioned incident Secret per workload/container. The Pods restart column opens a compact incident view with summary, T-3m/T-1m/T-30s/T-10s metrics, requests and limits, pod usage on the node, relevant events, and bounded timestamped previous logs for all regular and init containers.

### Files changed

- `internal/config/config.go`
- `internal/kube/restart_diagnostics.go`
- `internal/kube/metrics.go`
- `internal/kube/resources.go`
- `internal/kube/client_base.go`
- `internal/api/server.go`
- `internal/api/resource_handlers.go`
- `cmd/server/main.go`
- `ui/src/App.jsx`
- `ui/src/components/PodsPage.jsx`
- `ui/src/components/RestartDiagnosticModal.jsx`
- `ui/src/styles.css`
- `charts/beaverdeck/values.yaml`
- `charts/beaverdeck/values.schema.json`
- `charts/beaverdeck/templates/deployment.yaml`
- `README.md`
- `charts/beaverdeck/README.md`
- diagnostic/config/API tests and local project memory

### Reason

Operators need a lightweight incident snapshot that reconstructs the minutes immediately before a restart without making a monitoring stack or durable backend mandatory.

### Validation

- Covered restart transitions, watch-update deduplication, ignored deletion, eviction, ring lookup, partial history, Metrics Server/unavailable kubelet paths, JSON versioning, Secret overwrite, init and regular containers, missing previous logs, event filtering, workload ownership, and forbidden namespaces.
- `go test ./...`
- `npm run build` from `ui/`
- `helm lint charts/beaverdeck`
- `helm template beaverdeck charts/beaverdeck --namespace beaverdeck`
- `git diff --check`

### Follow-ups

- Consider an operator-facing retention/cleanup policy for unmanaged Pod snapshots if clusters use many bare Pods.

## 2026-07-16 — Prepare BeaverDeck 1.5.2 Security Release

### Summary

Prepared BeaverDeck `1.5.2` and Helm chart `2.2.3`, updated the vulnerable Go extension modules reported for
CVE-2026-46600 and CVE-2026-56852, synchronized release-facing version references, and added release notes.

### Files changed

- `go.mod`
- `go.sum`
- `ui/package.json`
- `ui/package-lock.json`
- `charts/beaverdeck/Chart.yaml`
- `charts/beaverdeck/values.yaml`
- `charts/beaverdeck/README.md`
- `README.md`
- `docs/changelog/1.5.2.md`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/DECISIONS.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

Security reports flagged `golang.org/x/net` `0.55.0` and `golang.org/x/text` `0.37.0`. The release raises them to
the reported fixed versions, `0.56.0` and `0.39.0`, and updates the default image tag and release metadata.

### Validation

- `go list -m golang.org/x/net golang.org/x/text golang.org/x/sys golang.org/x/term` reported `v0.56.0`, `v0.39.0`, `v0.46.0`, and `v0.44.0`.
- `go mod verify` passed.
- `go test ./...` passed.
- `go vet ./...` passed.
- `npm run build` passed for `beaverdeck-ui@1.5.2`; embedded assets were unchanged.
- `helm lint --strict charts/beaverdeck` passed.
- `helm template beaverdeck charts/beaverdeck` rendered chart `2.2.3`, app version `1.5.2`, image `arequs/beaverdeck:1.5.2`, and `APP_VERSION=1.5.2`.
- `govulncheck ./...` reported no vulnerabilities.
- `govulncheck -mode=binary /private/tmp/beaverdeck-1.5.2-server` reported no vulnerabilities.
- `git diff --check` passed.

### Follow-ups

- Publish the `1.5.2` image and `2.2.3` chart after review.

## 2026-06-30 — Link Every Insight To Its Documentation

### Summary

Added a compact Info button to every current Insight card. Hovering shows `Why is this an issue?`; clicking opens the
matching `beaverdeck.io` Insights Guide page in a new tab. Added responsive width and action wrapping rules so the
corner button and card content remain available on narrow screens.

### Files changed

- `ui/src/lib/insightDocumentation.js`
- `ui/src/components/InsightsPage.jsx`
- `ui/src/styles.css`
- rebuilt `cmd/server/web/dist/`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

Insight findings need a direct path to the explanation, risk, and remediation guidance maintained on the public
product documentation site.

### Validation

- Verified all 37 backend check types map exactly once and every mapped route has a local documentation page.
- Verified `node-condition` maps to `https://beaverdeck.io/docs/insights-guide/nodes/node-conditions/`.
- `npm run build` passed and regenerated embedded frontend assets.
- `go test ./...` passed.
- Inspected the real component with a temporary Vite fixture at desktop and mobile widths. The first mobile pass
  exposed action overflow; responsive wrapping and width constraints were added. Final DOM inspection confirmed the
  Info button, tooltip, and fixed mapping; a final repeat screenshot was interrupted by the local Chrome process.

### Follow-ups

- None.

## 2026-06-29 — Prepare Helm Chart 2.2.1

### Summary

Prepared Helm chart `2.2.1`, removed the Pricing Policy from the chart description, corrected the swapped Overview and Insights screenshots in both repository and chart documentation, and updated current install commands to the new chart version. BeaverDeck application version and image tag remain `1.5.0`.

### Files changed

- `README.md`
- `charts/beaverdeck/README.md`
- `charts/beaverdeck/Chart.yaml`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

The Artifact Hub chart description should omit the pricing statement, show screenshots in the sections they represent, and publish as patch chart `2.2.1` without changing the application release.

### Validation

- `helm lint --strict charts/beaverdeck` passed.
- `helm template` rendered chart labels as `beaverdeck-2.2.1` while retaining application version and image tag `1.5.0`.
- `helm package` created `/private/tmp/beaverdeck-2.2.1.tgz`.
- `helm show chart` and `helm show readme` confirmed packaged metadata, release annotations, screenshot placement, install version, and removal of the chart Pricing Policy section.

### Follow-ups

- None.

## 2026-06-29 — Paid Tier Readiness Assessment

### Summary

Assessed BeaverDeck and BeaverDeck-server for a planned free tier limited to five worker nodes with paid tiers for larger clusters. Recorded the planned direction, current reusable foundations, missing entitlement and billing capabilities, update-check contract mismatch, worker-counting concerns, and licensing-policy risks. No application or server code was changed.

### Files changed

- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

Future paid-version work needs persistent context about the intended node-based tiering model and the technical, product, security, and licensing decisions that must be made before implementation.

### Validation

- Inspected BeaverDeck configuration, Kubernetes Node access, API authorization boundaries, update-check implementation, Helm RBAC, pricing policy, and Apache-2.0 license.
- Inspected BeaverDeck-server API, configuration, tests, Dockerfile, local JSON store, and authentication behavior.
- `go test ./...` passed in BeaverDeck.
- `go test ./...` passed in BeaverDeck-server.
- `go vet ./...` passed in BeaverDeck-server.

### Follow-ups

- Resolve whether the node cap applies to existing functionality or only future commercial functionality.
- Define worker-node counting, grace/offline behavior, and over-limit UX.
- Align the update-check contract before using the server as a licensing foundation.
- Raise the BeaverDeck-server container build from Go `1.26.3` to the fixed Go `1.26.5` security baseline before exposing new commercial endpoints.
- Design a separate activation/entitlement protocol with server-signed offline-verifiable leases and production-grade durable storage.

## 2026-06-29 — Separate Helm Home And Source URLs

### Summary

Changed the Helm chart home page to the BeaverDeck product page and listed GitHub separately as the chart source repository.

### Files changed

- `charts/beaverdeck/Chart.yaml`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

Helm chart metadata should distinguish the public product page from the source-code repository.

### Validation

- `helm lint --strict charts/beaverdeck` passed.
- `helm package` regenerated `/private/tmp/beaverdeck-2.2.0.tgz`.
- `helm show chart` confirmed the separate `home` and `sources` values.

### Follow-ups

- None.

## 2026-06-29 — Prepare BeaverDeck 1.5.0 Release

### Summary

Prepared application `1.5.0` and Helm chart `2.2.0`, synchronized release-facing version references, replaced Artifact Hub change annotations, and added release notes covering features, fixes, security updates, and the breaking SQLite-to-Secret auth storage change.

### Files changed

- `ui/package.json`
- `ui/package-lock.json`
- `charts/beaverdeck/Chart.yaml`
- `charts/beaverdeck/values.yaml`
- `charts/beaverdeck/README.md`
- `README.md`
- `docs/changelog/1.5.0.md`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

The current feature and security work needed consistent release metadata, operator-facing upgrade warnings, packaged chart validation, and final release gates before publishing BeaverDeck `1.5.0`.

### Validation

- `npm run build` passed for `beaverdeck-ui@1.5.0`.
- `npm audit --json` reported zero vulnerabilities; `npm outdated --json` returned no updates.
- `GOCACHE=/private/tmp/beaverdeck-go-cache go test ./...` passed.
- `GOCACHE=/private/tmp/beaverdeck-go-cache go vet ./...` passed.
- `go mod verify` passed.
- source and production-like binary `govulncheck` scans reported no vulnerabilities.
- `helm lint --strict charts/beaverdeck` passed.
- `helm template` rendered chart `2.2.0`, image/APP_VERSION `1.5.0`, the suppressed Insights ConfigMap, and updated RBAC.
- `helm package` created `/private/tmp/beaverdeck-2.2.0.tgz`.
- `docker build -t beaverdeck:1.5.0-release-check .` passed; the image runs as UID `65532`, uses `/app/beaverdeck`, and passed the `hash-password` smoke test.
- `git diff --check` passed for release metadata and release notes.

### Follow-ups

- Review and commit the release changes, then create the release tag and publish image/chart artifacts when ready.

## 2026-06-29 — Ignore Local Project Memory

### Summary

Added all Codex project-memory files to the root Git ignore rules while keeping the local files intact.

### Files changed

- `.gitignore`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/DECISIONS.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

Project memory is local Codex context and should not be published in the public repository.

### Validation

- `git check-ignore -v` confirmed all four memory files are ignored.
- `git status --short` no longer lists the memory files.

### Follow-ups

- None.

## 2026-06-29 — Go Security Report Remediation

### Summary

Updated `golang.org/x/net` from `0.54.0` to `0.55.0`, updated the related `golang.org/x/sys` module to `0.45.0`, and verified that the application and production-like binary use Go `1.26.5` and contain none of the reported vulnerabilities.

### Files changed

- `go.mod`
- `go.sum`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

The reported `x/net` CVEs affect versions before `0.55.0`; the reported standard-library CVEs affect Go `1.26.x` before `1.26.5`.

### Validation

- `go list -m golang.org/x/net golang.org/x/sys` reported `v0.55.0` and `v0.45.0`.
- `go version` reported `go1.26.5`; the Docker build stage also uses `golang:1.26.5-alpine`.
- `govulncheck ./...` reported `No vulnerabilities found`.
- `govulncheck -mode=binary` reported `No vulnerabilities found` for both the normal and `CGO_ENABLED=0` binaries.
- `GOCACHE=/private/tmp/beaverdeck-go-cache go test ./...` passed.
- `GOCACHE=/private/tmp/beaverdeck-go-cache go vet ./...` passed.
- `git diff --check` passed.

### Follow-ups

- None.

## 2026-06-29 — Dependency Updates And Manifest Edit Permissions

### Summary

Updated available direct Go and frontend dependencies, removed the redundant Apply YAML permission requirement from existing-manifest editing, and added coverage proving role changes are resolved for both existing and newly issued sessions.

### Files changed

- `go.mod`
- `go.sum`
- `ui/package.json`
- `ui/package-lock.json`
- `internal/api/mutation_handlers.go`
- `internal/api/mutation_handlers_test.go`
- `internal/users/config_snapshot_test.go`
- `ui/src/components/BottomDock.jsx`
- `ui/src/components/NodesPage.jsx`
- `ui/src/components/RbacPages.jsx`
- `ui/src/components/ResourcePages.jsx`
- `ui/src/components/WorkloadsPage.jsx`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

Dependencies had available patch/minor updates, and manifest editing incorrectly required both resource edit access and the broader Apply YAML permission.

### Validation

- `npm outdated --json` returned no remaining direct updates.
- `npm audit --json` reported zero vulnerabilities.
- `npm run build` from `ui/` passed with Vite `8.1.0`.
- `GOCACHE=/private/tmp/beaverdeck-go-cache go test ./...` passed.
- `GOCACHE=/private/tmp/beaverdeck-go-cache go vet ./...` passed.
- `helm lint charts/beaverdeck` passed.
- `git diff --check` passed.

### Follow-ups

- None.

## 2026-06-29 — Expand User Guide Purpose

### Summary

Expanded the website User Guide Purpose page to explain BeaverDeck's inspection, triage, configuration-risk detection, capacity-planning, error-reduction, and GPU-efficiency goals. Clarified that GPU Insights inform operator decisions rather than automatically optimizing scheduling.

### Files changed

- `../BeaverDeck-website/beaverdeck/docs/user-guide/index.html`
- `docs/PROJECT_CONTEXT.md`
- `docs/CHANGELOG_AI.md`

### Reason

The Purpose page needed to describe the broader operational value of Insights and explain how BeaverDeck helps teams plan scarce and expensive GPU capacity.

### Validation

- Parsed the Purpose HTML and verified its heading structure.
- Verified all internal links on the Purpose page resolve.
- `git diff --check -- beaverdeck/docs/user-guide/index.html` passed in the website repository.

### Follow-ups

- None.

## 2026-06-29 — Remove Updates And Health Documentation Section

### Summary

Removed the standalone Update Checks, Logging, Metrics, and Health page and its navigation, pagination, sitemap, and internal-map references. Moved the public `/healthz` behavior and verification command to the installation verification guide.

### Files changed

- `../BeaverDeck-website/beaverdeck/docs/configuration-guide/updates-health/index.html` (removed)
- `../BeaverDeck-website/beaverdeck/docs/configuration-guide/verify-install/index.html`
- `../BeaverDeck-website/beaverdeck/docs/configuration-guide/rbac/index.html`
- `../BeaverDeck-website/beaverdeck/docs/configuration-guide/backup-restore/index.html`
- `../BeaverDeck-website/beaverdeck/docs/**/index.html` (sidebar navigation)
- `../BeaverDeck-website/docs/internal/application-configuration-map.md`
- `../BeaverDeck-website/sitemap.xml`
- `docs/CHANGELOG_AI.md`

### Reason

The standalone operational-detail section was not needed in the public documentation. Health verification belongs with post-installation checks.

### Validation

- Confirmed no `updates-health` links or removed section titles remain in the website repository.
- Parsed all 26 remaining documentation pages and verified their internal links resolve.
- Parsed `sitemap.xml` successfully.
- `git diff --check` passed in the website repository.

### Follow-ups

- None.

## 2026-06-29 — Default Helm RBAC Documentation

### Summary

Expanded the website Kubernetes RBAC guide with the exact default ClusterRole resources and verbs, the application features that use them, cluster-scope and custom-RBAC guidance, and the currently missing pod eviction permission.

### Files changed

- `../BeaverDeck-website/beaverdeck/docs/configuration-guide/rbac/index.html`
- `docs/PROJECT_CONTEXT.md`
- `docs/CHANGELOG_AI.md`

### Reason

Operators need to understand what access the default Helm chart grants to BeaverDeck and why each permission exists before accepting or replacing the cluster-wide policy.

### Validation

- Compared all 32 documented API group/resource permissions and verbs with `charts/beaverdeck/templates/rbac.yaml`.
- Parsed the updated HTML with the Python standard-library HTML parser.
- `git diff --check -- beaverdeck/docs/configuration-guide/rbac/index.html` passed in the website repository.

### Follow-ups

- Decide whether the chart should grant `create` on `policy` `pods/eviction` by default for pod Evict and node Drain.

## 2026-06-22 — Config Import Auth Provider Refresh

### Summary

Made admin configuration import refresh public auth provider metadata before signing the user out, so the login screen reflects imported Google/OIDC settings immediately. Added regression coverage proving config snapshot import overwrites both Google and generic OIDC config and mappings instead of keeping edited runtime values.

### Files changed

- `ui/src/App.jsx`
- `internal/users/config_snapshot_test.go`
- `docs/CHANGELOG_AI.md`

### Reason

Importing an exported configuration after changing Google/OIDC settings could look like it did not overwrite the auth config because the frontend preserved stale provider metadata during the forced sign-out flow.

### Validation

- `gofmt -w internal/users/config_snapshot_test.go` passed.
- `go test ./internal/users` passed.
- `GOCACHE=/private/tmp/beaverdeck-go-cache go test ./...` passed.
- `npm run build` from `ui/` passed.
- `git diff --check -- ui/src/App.jsx cmd/server/web/dist` passed.

### Follow-ups

- None.

## 2026-06-22 — Insight Loading And Sensitive Env Follow-up

### Summary

Confirmed sensitive environment variable detection uses contains-style matching equivalent to wildcard checks such as `*TOKEN*` and `*SECRET*`, added regression coverage for that behavior, and made Workload Insights load node data only when metrics fallback actually needs kubelet/node scraping.

### Files changed

- `internal/kube/insights.go`
- `internal/kube/insights_test.go`
- `internal/kube/metrics.go`
- `docs/CHANGELOG_AI.md`

### Reason

The user wanted confirmation that sensitive env checks match embedded tokens and asked to recheck whether opening an Insights section loads unrelated Kubernetes data.

### Validation

- `gofmt -w internal/kube/insights.go internal/kube/metrics.go internal/kube/insights_test.go` passed.
- `go test ./internal/kube` passed.
- `go test ./...` passed.

### Follow-ups

- None.

## 2026-06-22 — Insight Category Rework And New Checks

### Summary

Removed the separate Capacity Insights navigation entry, moved pod resource checks to Workload Insights, moved node capacity checks to Node Insights, added node underutilization detection below 50% requested CPU and memory, added GPU capacity discovery for clusters with GPU requests but no detected GPU nodes, and added Security Insights checks for missing namespace NetworkPolicy coverage and sensitive literal environment variables.

### Files changed

- `README.md`
- `charts/beaverdeck/README.md`
- `internal/kube/insights.go`
- `internal/kube/insights_test.go`
- `internal/kube/metrics.go`
- `ui/src/lib/appConstants.js`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

The Insights UI should no longer expose a separate Capacity section, capacity-related checks should be grouped with the workload/node areas they belong to, and GPU/Security/Node Insights needed additional actionable checks.

### Validation

- `gofmt -w internal/kube/insights.go internal/kube/insights_test.go internal/kube/metrics.go` passed.
- `go test ./internal/kube` passed.
- `npm run build` from `ui/` passed.
- `go test ./...` initially failed while running in parallel with `npm run build` because embedded frontend assets were being regenerated, then passed when rerun after the frontend build completed.

### Follow-ups

- Consider adding more precise NetworkPolicy coverage checks that match pod labels against policy selectors if operators need per-workload policy findings.

## 2026-06-22 — Dock And Navigation Follow-up

### Summary

Adjusted the decoded Secret panel to stay inside the manifest content area during bottom-dock resizing, strengthened navigation section title hover overrides, and made Local Users actions use the same `actions-cell` anchoring as group mapping tables.

### Files changed

- `ui/src/components/BottomDock.jsx`
- `ui/src/components/UserManagementPage.jsx`
- `ui/src/styles.css`
- `docs/CHANGELOG_AI.md`

### Reason

The decoded Secret panel reduced the visible manifest area during resize, navigation section titles still inherited generic button hover styles, and Local Users actions were still using a custom wrapper instead of the group-table action cell pattern.

### Validation

- `npm run build` from `ui/` passed.
- `git diff --check -- ui/src/components/BottomDock.jsx ui/src/components/UserManagementPage.jsx ui/src/styles.css docs/CHANGELOG_AI.md` passed.

### Follow-ups

- None.

## 2026-06-21 — Decoded Secret Reveal And Compact Navigation

### Summary

Reduced navigation menu vertical spacing, made navigation sections collapsed by default, removed hover highlighting from navigation section titles, centered Local Users table actions, and added Reveal Secret as a decoded-data panel while leaving Secret manifests in their normal base64 form.

### Files changed

- `internal/api/resource_handlers.go`
- `internal/kube/operations.go`
- `internal/kube/operations_test.go`
- `ui/src/App.jsx`
- `ui/src/components/BottomDock.jsx`
- `ui/src/lib/appConstants.js`
- `ui/src/styles.css`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

The sidebar navigation was too tall, navigation sections should start collapsed, Local Users actions were right-aligned, and Reveal Secret should show `base64 -d` values rather than just showing base64 manifest data.

### Validation

- `gofmt -w internal/api/resource_handlers.go internal/kube/operations.go internal/kube/operations_test.go`
- `git diff --check -- internal/api/resource_handlers.go internal/kube/operations.go internal/kube/operations_test.go ui/src/App.jsx ui/src/components/BottomDock.jsx ui/src/styles.css ui/src/lib/appConstants.js`
- `go test ./internal/kube`
- `go test ./internal/api`
- `go test ./...`
- `npm run build` from `ui/`

### Follow-ups

- None.

## 2026-06-21 — Compact Local Users Actions

### Summary

Adjusted the User Management Local Users table so row actions no longer force the table wider than the viewport.

### Files changed

- `ui/src/components/UserManagementPage.jsx`
- `ui/src/styles.css`

### Reason

The Local Users and Roles users table could overflow horizontally because the Actions column contained the role selector plus Reset password and Delete buttons inline.

### Validation

- `npm run build` from `ui/` passed.

### Follow-ups

- None.

## 2026-06-17 — Initialize Project Memory

### Summary

Created persistent Codex project memory and agent instructions for future repository work. Filled the initial context from repository files, including README, Go module metadata, frontend package metadata, Dockerfile, Helm chart, runtime config, server entrypoint, and storage/auth code.

### Files changed

- `AGENTS.md`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

The user requested permanent project memory so future Codex tasks start from known project facts, constraints, and update rules.

### Validation

- Inspected repository structure with `ls`, `find`, and `rg --files`.
- Read `README.md`, `go.mod`, `ui/package.json`, `Dockerfile`, Helm chart files, `cmd/server/main.go`, config, API routing, user store, config snapshot, config Secret, suppressed Insights ConfigMap logic, and deploy helper.
- No application tests were run because this task only adds documentation/memory files.

### Follow-ups

- Keep these memory files updated after every future task.
- Clarify the authoritative CI/CD process; no workflow directory was found in this checkout.
## 2026-08-15 — Browse And Manage Custom Resources From CRDs

### Summary

Made the CRDs sidebar entry expandable, added dynamic discovery and tables for every installed custom resource type, and reused manifest, edit, and delete actions with server-side namespace enforcement.

### Files changed

- `internal/api/*`
- `internal/kube/*`
- `ui/src/*`
- `charts/beaverdeck/templates/rbac.yaml`
- `README.md`
- `charts/beaverdeck/README.md`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

CRDs needed to remain directly navigable while also exposing their instances, without leaking namespaced resources outside a user's BeaverDeck RBAC allowlist.

### Validation

- `go test ./...`
- `go vet ./...`
- `npm run build` from `ui/`
- `helm lint --strict charts/beaverdeck`
- `git diff --check`

### Follow-ups

- Review the chart's wildcard custom-resource ClusterRole grant against each deployment's trust model.
## 2026-08-15 — Align Sidebar Navigation As A Tree

### Summary

Moved the CRDs expansion chevron to the left and applied the same indented tree styling to every navigation item below its section heading.

### Files changed

- `ui/src/components/AppChrome.jsx`
- `ui/src/styles.css`
- `docs/CHANGELOG_AI.md`

### Reason

CRDs and the other items under Config, RBAC, Networking, and the remaining sidebar sections needed consistent hierarchy, alignment, and visual guides.

### Validation

- `npm run build` from `ui/`
- `git diff --check`

### Follow-ups

- None.
## 2026-08-15 — Add CRD Apply YAML Examples

### Summary

Added paired Apply YAML templates for a namespaced sample Widget CRD and one Widget instance.

### Files changed

- `ui/src/lib/appConstants.js`
- `docs/PROJECT_CONTEXT.md`
- `docs/CHANGELOG_AI.md`

### Reason

Users need a small example that exercises CRD discovery and the custom-resource Manifest, Edit, and Delete workflow.

### Validation

- `npm run build` from `ui/`
- `git diff --check`

### Follow-ups

- Apply the CRD template before the custom-resource template because Kubernetes must register the new API first.
## 2026-08-15 — Reduce Sidebar Leaf Indentation

### Summary

Removed the empty expansion-control spacing from regular sidebar items while preserving the CRD subtree indentation.

### Files changed

- `ui/src/components/AppChrome.jsx`
- `ui/src/styles.css`
- `docs/CHANGELOG_AI.md`

### Reason

Regular section items had substantially more left whitespace than needed.

### Validation

- `npm run build` from `ui/`
- `git diff --check`

### Follow-ups

- None.
## 2026-08-15 — Allow Custom Resource Creation In Helm RBAC

### Summary

Added the missing `create` verb to the wildcard ClusterRole rule used for arbitrary custom resources.

### Files changed

- `charts/beaverdeck/templates/rbac.yaml`
- `README.md`
- `charts/beaverdeck/README.md`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

The chart explicitly allowed CRD definition creation but did not include `create` in the generic rule needed for instances of newly installed custom resource types.

### Validation

- `helm template beaverdeck charts/beaverdeck --namespace beaverdeck`
- `helm lint --strict charts/beaverdeck`
- `git diff --check`

### Follow-ups

- Upgrade the Helm release so the deployed ClusterRole receives the new verb.
## 2026-08-15 — Group CRD Navigation By API Group

### Summary

Added an API-group level between CRDs and individual CRD kinds in the sidebar. Groups can be selected to filter the definitions table and expanded independently to reveal their kinds.

### Files changed

- `ui/src/App.jsx`
- `ui/src/components/AppChrome.jsx`
- `ui/src/components/ResourcePages.jsx`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

Clusters with many CRDs need a compact hierarchy organized by API group while retaining direct access to both definitions and their instances.

### Validation

- `npm run build` from `ui/`
- `git diff --check`

### Follow-ups

- None.
## 2026-08-16 — Compact CRD Tree Geometry

### Summary

Reduced each CRD navigation level to the same compact indentation rhythm as the top-level sections and aligned every vertical guide beneath its parent chevron.

### Files changed

- `ui/src/components/AppChrome.jsx`
- `ui/src/styles.css`
- `docs/CHANGELOG_AI.md`

### Reason

Nested API groups and CRD kinds accumulated excessive left whitespace, and their vertical guides did not align with the expansion controls.

### Validation

- `npm run build` from `ui/`
- `git diff --check`

### Follow-ups

- None.
## 2026-08-16 — Align Top-Level Navigation Guide

### Summary

Aligned the section-level vertical navigation guide beneath its chevron so it follows the same 15-pixel nesting rhythm as the CRD tree.

### Files changed

- `ui/src/styles.css`
- `docs/CHANGELOG_AI.md`

### Reason

The top-level guide under headings such as Config was offset to the left and visually inconsistent with the correctly aligned nested guides.

### Validation

- `npm run build` from `ui/`
- `git diff --check`

### Follow-ups

- None.
## 2026-08-16 — Add Helm Releases And History

### Summary

Added a Helm → Releases workspace with namespace-scoped release discovery, status, chart metadata, and revision history. Added None, View (default for new roles), and Manage permission choices.

### Files changed

- `internal/kube/client_base.go`
- `internal/kube/helm_releases.go`
- `internal/kube/helm_releases_test.go`
- `internal/api/resource_handlers.go`
- `internal/api/resource_handlers_test.go`
- `internal/api/server.go`
- `ui/src/App.jsx`
- `ui/src/components/HelmReleasesPage.jsx`
- `ui/src/lib/appConstants.js`
- `ui/src/styles.css`
- `README.md`
- `charts/beaverdeck/README.md`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

Operators need an initial Helm release inventory and history view that follows BeaverDeck namespace RBAC before release mutations are introduced.

### Validation

- `go test ./...`
- `go vet ./...`
- `npm run build` from `ui/`
- `helm lint --strict charts/beaverdeck`
- `git diff --check`

### Follow-ups

- Define concrete Manage actions such as rollback, uninstall, and upgrade after the read-only workflow is reviewed.
- SQL-backed Helm storage is not supported by this implementation.
## 2026-08-16 — Nest Helm Releases Under Applications And Add Revision Details

### Summary

Renamed the Helm navigation section to Applications, made Helm Releases expandable into installed releases, and made release selection open revision history. Added Manage-only Values, User-applied values, and Created resources actions rendered in read-only YAML bottom-dock tabs.

### Files changed

- `internal/kube/helm_releases.go`
- `internal/kube/helm_releases_test.go`
- `internal/api/resource_handlers.go`
- `internal/api/resource_handlers_test.go`
- `internal/api/server.go`
- `ui/src/App.jsx`
- `ui/src/components/AppChrome.jsx`
- `ui/src/components/BottomDock.jsx`
- `ui/src/components/HelmReleasesPage.jsx`
- `ui/src/lib/appConstants.js`
- `README.md`
- `charts/beaverdeck/README.md`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

Application delivery should have a scalable navigation hierarchy, while Helm Manage needs safe inspection of the exact configuration and resources stored for each revision.

### Validation

- `go test ./...`
- `go vet ./...`
- `npm run build` from `ui/`
- `helm lint --strict charts/beaverdeck`
- `git diff --check`

### Follow-ups

- Consider Argo CD Applications, Flux resources, Kustomize inventories, and OLM subscriptions as additional Applications children.
- Define mutation workflows separately before adding rollback, upgrade, or uninstall.
## 2026-08-16 — Fix Applications Selection And Section-Wide RBAC

### Summary

Removed duplicate parent/child highlighting for selected Helm releases, aligned child active bounds with Helm Releases, cleared release selection when leaving Applications, removed namespace suffixes from release labels, and renamed the `helm` permission to section-wide `applications` access.

### Files changed

- `internal/api/resource_handlers.go`
- `internal/api/resource_handlers_test.go`
- `ui/src/App.jsx`
- `ui/src/components/AppChrome.jsx`
- `ui/src/components/HelmReleasesPage.jsx`
- `ui/src/lib/appConstants.js`
- `ui/src/styles.css`
- `README.md`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

The navigation had conflicting active states and Helm-specific authorization would not scale to future Argo CD, Flux, or other Applications children.

### Validation

- `go test ./...`
- `go vet ./...`
- `npm run build` from `ui/`
- `git diff --check`

### Follow-ups

- None.
## 2026-08-16 — Default Missing Applications Permission To None

### Summary

Changed Applications access to least-privilege defaults: new roles and roles saved by older BeaverDeck versions show Applications as None until explicitly granted. Renamed `View (default)` to `View`.

### Files changed

- `ui/src/App.jsx`
- `ui/src/lib/appConstants.js`
- `ui/src/lib/appUtils.js`
- `README.md`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

### Reason

Introducing a new permission must not silently grant it to existing roles, and the role editor must still surface the new Applications row when the stored permission object does not contain it.

### Validation

- Verified legacy permission normalization produces `applications: none`.
- `npm run build` from `ui/`
- `go test ./...`
- `git diff --check`

### Follow-ups

- None.

## 2026-08-16 — Add Argo CD Applications And Deployment History

### Summary

Added Argo CD Applications as a second expandable provider under Applications. The UI now shows application sync and health status, project, source, destination, current revision, and deployment history. Applications Manage users can open a revision's source configuration and the current revision's created-resource inventory in read-only YAML tabs.

### Files changed

- `internal/kube/argocd_applications.go`
- `internal/kube/argocd_applications_test.go`
- `internal/kube/client_base.go`
- `internal/api/resource_handlers.go`
- `internal/api/resource_handlers_test.go`
- `internal/api/server.go`
- `ui/src/App.jsx`
- `ui/src/components/AppChrome.jsx`
- `ui/src/components/ArgoCDApplicationsPage.jsx`
- `ui/src/lib/appConstants.js`
- `ui/src/styles.css`
- rebuilt `cmd/server/web/dist/`
- `README.md`
- `charts/beaverdeck/README.md`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/DECISIONS.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

Operators need Argo CD application visibility beside Helm releases while keeping the same Applications permission and namespace model.

### Validation

- Added Kubernetes parsing tests for application inventory, history ordering, current-revision detection, source details, created-resource details, namespace isolation, and absent-CRD behavior.
- Added API authorization tests for forbidden namespaces and Manage-only revision details.
- `go test ./...`
- `npm run build` from `ui/`
- `helm lint --strict charts/beaverdeck`
- `helm template beaverdeck charts/beaverdeck --namespace beaverdeck`
- `git diff --check`

### Follow-ups

- Add sync, refresh, rollback, or delete only as separately reviewed mutation workflows.

## 2026-08-17 — Align Selected CRD Highlight Bounds

### Summary

Applied the shared nested-leaf geometry to CRD kind entries so their active and hover bounds align with the parent API-group row, matching Helm release and Argo CD application leaves.

### Files changed

- `ui/src/components/AppChrome.jsx`
- `ui/src/styles.css`
- rebuilt `cmd/server/web/dist/`
- `docs/PROJECT_CONTEXT.md` (local-only)
- `docs/CHANGELOG_AI.md` (local-only)

### Reason

Selected CRD kinds extended three pixels farther right than their parent row and looked wider when highlighted.

### Validation

- `npm run build` from `ui/`
- `git diff --check`

### Follow-ups

- None.

## 2026-08-17 — Security, Persistence, And Resource-Bound Maintenance Pass

### Summary

Closed the Apply YAML cross-namespace authorization gap, removed the unused `logMutation` audit stub and all calls,
made auth configuration mutations atomic with Secret persistence, bounded aggregate/API/log/Helm resource usage,
hardened Helm defaults, updated safe patch/security dependencies, and removed a duplicate Cluster Health Pods call.

### Files changed

- `internal/api/*` mutation, resource, server, and persistence-import handlers and tests
- `internal/users/*` config mutation paths and tests
- `internal/kube/operations.go`, `internal/kube/helm_releases.go`, and tests
- `ui/src/App.jsx`, `ui/package-lock.json`
- `go.mod`, `go.sum`
- `openapi.yaml`
- `charts/beaverdeck/templates/deployment.yaml`, `templates/rbac.yaml`, values, schema, and chart README
- `docs/PROJECT_CONTEXT.md`, `docs/DECISIONS.md`, `docs/CHANGELOG_AI.md` (local-only)

### Reason

The audit identified one direct RBAC bypass, durable/runtime split-brain risks, unbounded request amplification paths,
vulnerable build dependencies, and chart defaults that were less portable or hardened than necessary.

### Validation

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go mod verify`
- `govulncheck ./...` — no vulnerabilities
- `npm audit` — no vulnerabilities
- `npm run build` from `ui/`
- `helm lint charts/beaverdeck`
- `helm template beaverdeck charts/beaverdeck --set persistence.enabled=true`
- `git diff --check`

### Follow-ups

- Expand `openapi.yaml`; its version and audit path are corrected, but most current routes remain undocumented.
- Add a frontend test/lint stack before doing broader UI concurrency refactors.

## 2026-08-29 — Isolate OIDC And Entra Configuration

### Summary

Separated generic OpenID Connect and Azure Entra ID across persistent configuration, mappings, admin APIs, OAuth state, role resolution, external sessions, login buttons, and Admin UI state. Added automatic migration for legacy Entra settings stored in the shared OIDC snapshot slot while preserving the historical callback URI.

### Files changed

- `internal/users/*` provider storage, snapshot migration, mappings, sessions, and tests
- `internal/api/*` provider discovery, admin routes, login flows, callback dispatch, and tests
- `ui/src/App.jsx`, auth hook, login/admin components, and rebuilt embedded assets
- `README.md` and `docs/changelog/1.6.1.md`
- project memory files

### Reason

Configuring or disabling one provider overwrote the other because the visual separation was not backed by independent runtime and durable state.

### Validation

- `go test ./...`
- `go test -race ./internal/users ./internal/api`
- `go vet ./...`
- `npm run build` from `ui/`
- `git diff --check`

### Follow-ups

- None.

## 2026-08-29 — Prepare BeaverDeck 1.6.1 And Helm Chart 2.2.6

### Summary

Reclassified the complete current worktree as BeaverDeck `1.6.1`, created a dedicated patch-release changelog, and synchronized application, frontend, OpenAPI, Helm chart, default image, examples, and Artifact Hub metadata. Restored the existing `1.6.0` changelog to its released state.

### Files changed

- `docs/changelog/1.6.1.md`
- `openapi.yaml`
- `ui/package.json`, `ui/package-lock.json`
- `charts/beaverdeck/Chart.yaml`, `values.yaml`, and `README.md`
- `README.md`
- project memory files

### Reason

The current diff belongs to the patch release after 1.6.0, not to the already prepared 1.6.0 release.

### Validation

- `npm run build` passed as `beaverdeck-ui@1.6.1`
- `go test ./...`
- `helm lint --strict charts/beaverdeck`
- full `helm template` render passed; Deployment uses chart `2.2.6`, app/image/`APP_VERSION` `1.6.1`
- `helm package` created `/private/tmp/beaverdeck-2.2.6.tgz`; packaged metadata was verified
- active release metadata contains no stale `1.6.0` or `2.2.5` references
- `git diff --check`

### Follow-ups

- Publish application image `1.6.1` and Helm chart `2.2.6` after review.

## 2026-08-30 — Fix Restart Diagnostics, Add Resource Bulk Actions, And Reconnect Exec

### Summary

Constrained Restart Diagnostics to a stable responsive width, wrapped hostile log lines, and changed metric lookup to
use Kubernetes sample timestamps plus an immediate incident-time collection. Added reusable multi-selection and
sequential RBAC-aware bulk actions to resource tables with existing mutations. Added in-place reconnect for dropped
Pod Exec WebSockets while preserving the tab and terminal output.

### Files changed

- `internal/kube/restart_diagnostics.go` and tests
- `ui/src/App.jsx`, `ui/src/styles.css`, and `ui/src/components/BottomDock.jsx`
- resource/workload/RBAC pages, `ui/src/components/BulkSelection.jsx`, and `ui/src/hooks/useBulkSelection.js`
- `docs/changelog/1.6.1.md` and project memory files
- rebuilt embedded frontend assets under `cmd/server/web/dist`

### Reason

Long logs could distort the incident modal, delayed Metrics API responses were incorrectly discarded, interrupted
Exec sessions required opening a new tab, and non-Pod resource tables lacked the existing Pod bulk workflow.

### Validation

- `go test ./...`
- `npm run build` from `ui/`
- `git diff --check`

### Follow-ups

- Browser runtime discovery returned no available browser, so interactive visual verification still needs a connected
  Browser/Chrome session or deployment-side review.

## 2026-08-31 — Replace Follow Polling With Safe Log Streaming

### Summary

Changed Pod and Workload Follow from repeated full-snapshot polling to one authenticated SSE connection. Workload
streams are multiplexed with source Pod labels. The server now sends heartbeats, disables compatible ingress
buffering, JSON-encodes events, and truncates then drains individual lines above 1 MiB without losing the following
logs. The browser cancels inactive streams, resumes without replaying history, batches high-volume updates, and keeps
bounded per-tab content and frame buffers.

### Files changed

- `internal/api/log_stream.go`, `internal/api/log_stream_test.go`, and log handlers
- `internal/kube/operations.go`
- `ui/src/App.jsx`, `ui/src/components/BottomDock.jsx`, and `ui/src/lib/api.js`
- `openapi.yaml`, `docs/changelog/1.6.1.md`, and project memory files
- rebuilt embedded frontend assets under `cmd/server/web/dist`

### Reason

Polling could overlap expensive responses and amplify client, server, and Kubernetes API memory pressure. The old
scanner-based SSE route failed with `bufio.Scanner: token too long` on a single oversized log entry.

### Validation

- focused API and Kubernetes package tests
- SSE headers, JSON event framing, end event, zero-tail reconnect, and long-line continuation regression tests
- `npm run build` from `ui/`
- `go test ./...`
- `go test -race ./internal/api ./internal/kube`
- `go vet ./...`
- `go mod verify`
- `helm lint --strict charts/beaverdeck`
- `helm template beaverdeck charts/beaverdeck --set persistence.enabled=true`
- `git diff --check`

### Follow-ups

- None.
