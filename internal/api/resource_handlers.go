package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"beaverdeck/internal/auth"
	"beaverdeck/internal/kube"
)

const (
	defaultLogTailLines         = 200
	maxLogTailLines             = 10_000
	maxLogStreamLineBytes       = 1024 * 1024
	maxNamespaceListConcurrency = 8
)

func (s *Server) namespaces(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AllowAllNamespaces {
		items := []string{s.cfg.ManagedNamespace}
		if !auth.IsAdmin(r.Context()) && !s.isNamespaceAllowedByRole(r, s.cfg.ManagedNamespace) {
			items = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}

	items, err := s.kube.ListNamespaces(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !auth.IsAdmin(r.Context()) {
		filtered := make([]string, 0, len(items))
		for _, ns := range items {
			if s.isNamespaceAllowedByRole(r, ns) {
				filtered = append(filtered, ns)
			}
		}
		items = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) workloads(w http.ResponseWriter, r *http.Request) {
	writeNamespacedList(s, w, r, "workloads", func(ctx context.Context, ns string) ([]kube.Workload, error) {
		return s.kube.ListWorkloads(ctx, ns)
	}, func(a, b kube.Workload) bool {
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
}

func (s *Server) pods(w http.ResponseWriter, r *http.Request) {
	includeMetrics := r.URL.Query().Get("include_metrics") == "1"
	writeNamespacedList(s, w, r, "pods", func(ctx context.Context, ns string) ([]kube.PodInfo, error) {
		return s.kube.ListPods(ctx, ns, includeMetrics)
	}, func(a, b kube.PodInfo) bool {
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
}

func (s *Server) restartDiagnosticSummaries(w http.ResponseWriter, r *http.Request) {
	writeNamespacedList(s, w, r, "pods", func(ctx context.Context, ns string) ([]kube.RestartDiagnosticSummary, error) {
		return s.kube.ListRestartDiagnosticSummaries(ctx, ns)
	}, func(a, b kube.RestartDiagnosticSummary) bool {
		return a.IncidentTime.After(b.IncidentTime)
	})
}

func (s *Server) restartDiagnosticSnapshot(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "pods", "view") {
		return
	}
	ns, ok := s.namespaceFromQuery(r)
	if !ok {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("diagnostic name is required"))
		return
	}
	snapshot, err := s.kube.GetRestartDiagnosticSnapshot(r.Context(), ns, name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) nodes(w http.ResponseWriter, r *http.Request) {
	writeClusterList(s, w, r, "nodes", func(ctx context.Context) ([]kube.NodeInfo, error) {
		return s.kube.ListNodes(ctx)
	})
}

func (s *Server) ingresses(w http.ResponseWriter, r *http.Request) {
	writeNamespacedList(s, w, r, "ingresses", func(ctx context.Context, ns string) ([]kube.IngressInfo, error) {
		return s.kube.ListIngresses(ctx, ns)
	}, func(a, b kube.IngressInfo) bool {
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
}

func (s *Server) secrets(w http.ResponseWriter, r *http.Request) {
	writeNamespacedList(s, w, r, "secrets", func(ctx context.Context, ns string) ([]kube.SecretInfo, error) {
		return s.kube.ListSecrets(ctx, ns)
	}, func(a, b kube.SecretInfo) bool {
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
}

func (s *Server) configMaps(w http.ResponseWriter, r *http.Request) {
	writeNamespacedList(s, w, r, "configmaps", func(ctx context.Context, ns string) ([]kube.ConfigMapInfo, error) {
		return s.kube.ListConfigMaps(ctx, ns)
	}, func(a, b kube.ConfigMapInfo) bool {
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
}

func (s *Server) helmReleases(w http.ResponseWriter, r *http.Request) {
	writeNamespacedList(s, w, r, "applications", func(ctx context.Context, ns string) ([]kube.HelmReleaseInfo, error) {
		return s.kube.ListHelmReleases(ctx, ns)
	}, func(a, b kube.HelmReleaseInfo) bool {
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
}

func (s *Server) helmReleaseHistory(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "applications", "view") {
		return
	}
	namespace, allowed := s.namespaceFromQuery(r)
	if !allowed {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("release name is required"))
		return
	}
	items, err := s.kube.ListHelmReleaseHistory(r.Context(), namespace, name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) helmReleaseRevisionDetail(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "applications", "edit") {
		return
	}
	namespace, allowed := s.namespaceFromQuery(r)
	if !allowed {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	revision, err := strconv.Atoi(strings.TrimSpace(r.PathValue("revision")))
	if name == "" || err != nil || revision < 1 {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("valid release name and revision are required"))
		return
	}
	detail := strings.TrimSpace(r.PathValue("detail"))
	content, err := s.kube.GetHelmReleaseRevisionYAML(r.Context(), namespace, name, revision, detail)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"yaml": content})
}

func (s *Server) argoCDApplications(w http.ResponseWriter, r *http.Request) {
	writeNamespacedList(s, w, r, "applications", func(ctx context.Context, ns string) ([]kube.ArgoCDApplicationInfo, error) {
		return s.kube.ListArgoCDApplications(ctx, ns)
	}, func(a, b kube.ArgoCDApplicationInfo) bool {
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
}

func (s *Server) argoCDApplicationHistory(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "applications", "view") {
		return
	}
	namespace, allowed := s.namespaceFromQuery(r)
	if !allowed {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("application name is required"))
		return
	}
	items, err := s.kube.ListArgoCDApplicationHistory(r.Context(), namespace, name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) argoCDApplicationRevisionDetail(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "applications", "edit") {
		return
	}
	namespace, allowed := s.namespaceFromQuery(r)
	if !allowed {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	revision, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("revision")), 10, 64)
	if name == "" || err != nil || revision < 0 {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("valid application name and history ID are required"))
		return
	}
	detail := strings.TrimSpace(r.PathValue("detail"))
	content, err := s.kube.GetArgoCDApplicationRevisionYAML(r.Context(), namespace, name, revision, detail)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"yaml": content})
}

func (s *Server) crds(w http.ResponseWriter, r *http.Request) {
	writeClusterList(s, w, r, "crds", func(ctx context.Context) ([]kube.CRDInfo, error) {
		return s.kube.ListCRDs(ctx)
	})
}

func (s *Server) customResources(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "crds", "view") {
		return
	}
	var namespaces []string
	var err error
	if strings.TrimSpace(r.URL.Query().Get("namespace")) != "" {
		namespaces, err = s.namespacedQuery(r)
		if err != nil {
			writeErr(w, http.StatusForbidden, err)
			return
		}
	}
	definition, err := s.kube.ResolveCustomResourceDefinition(r.Context(), r.PathValue("name"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if definition.Namespaced && len(namespaces) == 0 {
		namespaces, err = s.namespacedQuery(r)
		if err != nil {
			writeErr(w, http.StatusForbidden, err)
			return
		}
	}
	items, err := s.kube.ListCustomResources(r.Context(), definition, namespaces)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"definition": map[string]any{
			"name": definition.Name, "group": definition.Group, "version": definition.Version,
			"resource": definition.Resource, "kind": definition.Kind, "namespaced": definition.Namespaced,
		},
	})
}

func (s *Server) services(w http.ResponseWriter, r *http.Request) {
	writeNamespacedList(s, w, r, "services", func(ctx context.Context, ns string) ([]kube.ServiceInfo, error) {
		return s.kube.ListServices(ctx, ns)
	}, func(a, b kube.ServiceInfo) bool {
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
}

func (s *Server) clusterRoles(w http.ResponseWriter, r *http.Request) {
	writeClusterList(s, w, r, "clusterroles", func(ctx context.Context) ([]kube.ClusterRoleInfo, error) {
		return s.kube.ListClusterRoles(ctx)
	})
}

func (s *Server) clusterRoleBindings(w http.ResponseWriter, r *http.Request) {
	writeClusterList(s, w, r, "clusterroles", func(ctx context.Context) ([]kube.ClusterRoleBindingInfo, error) {
		return s.kube.ListClusterRoleBindings(ctx)
	})
}

func (s *Server) rbacRoles(w http.ResponseWriter, r *http.Request) {
	writeNamespacedList(s, w, r, "rbacroles", func(ctx context.Context, ns string) ([]kube.RoleInfo, error) {
		return s.kube.ListRoles(ctx, ns)
	}, func(a, b kube.RoleInfo) bool {
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
}

func (s *Server) rbacRoleBindings(w http.ResponseWriter, r *http.Request) {
	writeNamespacedList(s, w, r, "rbacroles", func(ctx context.Context, ns string) ([]kube.RoleBindingInfo, error) {
		return s.kube.ListRoleBindings(ctx, ns)
	}, func(a, b kube.RoleBindingInfo) bool {
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
}

func (s *Server) serviceAccounts(w http.ResponseWriter, r *http.Request) {
	writeNamespacedList(s, w, r, "serviceaccounts", func(ctx context.Context, ns string) ([]kube.ServiceAccountInfo, error) {
		return s.kube.ListServiceAccounts(ctx, ns)
	}, func(a, b kube.ServiceAccountInfo) bool {
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
}

func (s *Server) pvcs(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "pvcs", "view") {
		return
	}
	nsList, ok := s.namespacesFromQuery(r)
	if !ok {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	includeMetrics := r.URL.Query().Get("include_metrics") != "0"
	items, err := s.kube.ListPVCs(r.Context(), nsList, includeMetrics)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) pvs(w http.ResponseWriter, r *http.Request) {
	includeMetrics := r.URL.Query().Get("include_metrics") != "0"
	writeClusterList(s, w, r, "pvs", func(ctx context.Context) ([]kube.PVInfo, error) {
		return s.kube.ListPVs(ctx, includeMetrics)
	})
}

func (s *Server) storageClasses(w http.ResponseWriter, r *http.Request) {
	writeClusterList(s, w, r, "storageclasses", func(ctx context.Context) ([]kube.StorageClassInfo, error) {
		return s.kube.ListStorageClasses(ctx)
	})
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	eventType := strings.TrimSpace(r.URL.Query().Get("type"))
	writeNamespacedList(s, w, r, "events", func(ctx context.Context, ns string) ([]kube.EventInfo, error) {
		return s.kube.ListEventsByType(ctx, ns, limit, eventType)
	}, func(a, b kube.EventInfo) bool {
		return a.LastSeen > b.LastSeen
	}, func(items []kube.EventInfo) []kube.EventInfo {
		if limit > 0 && len(items) > limit {
			return items[:limit]
		}
		return items
	})
}

func (s *Server) insights(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "insights", "view") {
		return
	}
	nsList, ok := s.namespacesFromQuery(r)
	if !ok {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	items, err := s.kube.BuildInsights(r.Context(), nsList, category)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	suppressedKeys, err := s.kube.ListSuppressedInsights(r.Context(), s.suppressedInsightsRef())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	suppressedMap := make(map[string]struct{}, len(suppressedKeys))
	for _, key := range suppressedKeys {
		suppressedMap[key] = struct{}{}
	}

	out := make([]kube.InsightAlert, 0, len(items))
	alertCount := 0
	activeCount := 0
	okCount := 0
	suppressedCount := 0
	for _, item := range items {
		if item.Status == "alert" {
			alertCount++
		} else {
			okCount++
		}
		if _, ok := suppressedMap[item.Key]; ok {
			item.Suppressed = true
			if item.Status == "alert" {
				suppressedCount++
			}
		}
		if item.Status == "alert" && !item.Suppressed {
			activeCount++
		}
		out = append(out, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"category": category,
		"items":    out,
		"summary": map[string]any{
			"total":      len(items),
			"alerts":     alertCount,
			"active":     activeCount,
			"passing":    okCount,
			"suppressed": suppressedCount,
		},
	})
}

type setInsightSuppressedRequest struct {
	Key        string `json:"key"`
	Suppressed bool   `json:"suppressed"`
}

func (s *Server) setInsightSuppressed(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "insights", "edit") {
		return
	}
	var req setInsightSuppressedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.kube.SetInsightSuppressed(r.Context(), s.suppressedInsightsRef(), strings.TrimSpace(req.Key), req.Suppressed); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) suppressedInsightsRef() kube.SuppressedInsightsRef {
	return kube.SuppressedInsightsRef{
		Namespace: s.cfg.SuppressedInsightsConfigMapNS,
		Name:      s.cfg.SuppressedInsightsConfigMapName,
		Key:       s.cfg.SuppressedInsightsConfigMapKey,
	}
}

func manifestPermissionAction(resource string) string {
	if resource == "secrets" {
		return "edit"
	}
	return "view"
}

func (s *Server) manifest(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if kind == "" || name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("kind and name are required"))
		return
	}
	resource := kindToResource(kind)
	if resource == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("unsupported kind: %s", kind))
		return
	}
	if !s.requirePermission(w, r, resource, manifestPermissionAction(resource)) {
		return
	}
	ns := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if crdName, custom := customResourceCRDName(kind); custom {
		definition, err := s.kube.ResolveCustomResourceDefinition(r.Context(), crdName)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if definition.Namespaced && !s.namespaceAllowedForRequest(r, ns) {
			writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
			return
		}
	} else {
		var ok bool
		ns, ok = s.namespaceFromQuery(r)
		if !ok {
			writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
			return
		}
	}

	revealSecret := resource == "secrets" &&
		(strings.EqualFold(r.URL.Query().Get("reveal"), "1") ||
			strings.EqualFold(r.URL.Query().Get("reveal"), "true"))
	if revealSecret && !s.requirePermission(w, r, "secrets", "edit") {
		return
	}

	text, err := s.kube.GetManifestYAML(r.Context(), ns, kind, name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	response := map[string]any{"namespace": ns, "kind": kind, "name": name, "yaml": text, "revealed": false}
	if revealSecret {
		decoded, err := s.kube.GetSecretDecodedData(r.Context(), ns, name)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		response["revealed"] = true
		response["decoded_data"] = decoded
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) podLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "pods", "view") {
		return
	}
	ns, ok := s.namespaceFromQuery(r)
	if !ok {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	pod := strings.TrimSpace(r.URL.Query().Get("pod"))
	if pod == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("pod is required"))
		return
	}
	container := strings.TrimSpace(r.URL.Query().Get("container"))
	tail := requestedLogTail(r)
	follow := strings.EqualFold(r.URL.Query().Get("follow"), "1") || strings.EqualFold(r.URL.Query().Get("follow"), "true")

	if !follow {
		text, err := s.kube.PodLogs(r.Context(), ns, pod, container, tail)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(text))
		return
	}

	stream, err := s.kube.FollowPodLogs(r.Context(), ns, pod, container, requestedFollowLogTail(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	serveLogStreams(w, r, []namedLogStream{{stream: stream}})
}

func (s *Server) workloadLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "workloads", "view") {
		return
	}
	ns, ok := s.namespaceFromQuery(r)
	if !ok {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if kind == "" || name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("kind and name are required"))
		return
	}
	tail := requestedLogTail(r)
	follow := strings.EqualFold(r.URL.Query().Get("follow"), "1") || strings.EqualFold(r.URL.Query().Get("follow"), "true")
	if follow {
		streams, err := s.kube.FollowWorkloadLogs(r.Context(), ns, kind, name, requestedFollowLogTail(r))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if len(streams) == 0 {
			writeErr(w, http.StatusNotFound, fmt.Errorf("no pods found for %s/%s in namespace %s", kind, name, ns))
			return
		}
		sources := make([]namedLogStream, 0, len(streams))
		for _, stream := range streams {
			sources = append(sources, namedLogStream{source: stream.Pod, stream: stream.Stream, err: stream.Error})
		}
		serveLogStreams(w, r, sources)
		return
	}
	text, err := s.kube.WorkloadLogs(r.Context(), ns, kind, name, tail)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(text))
}

func requestedLogTail(r *http.Request) int64 {
	tail, _ := strconv.ParseInt(r.URL.Query().Get("tail"), 10, 64)
	if tail <= 0 {
		return defaultLogTailLines
	}
	if tail > maxLogTailLines {
		return maxLogTailLines
	}
	return tail
}

func requestedFollowLogTail(r *http.Request) int64 {
	raw := strings.TrimSpace(r.URL.Query().Get("tail"))
	if raw == "0" {
		return 0
	}
	return requestedLogTail(r)
}

func (s *Server) namespaceFromQuery(r *http.Request) (string, bool) {
	ns := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if ns == "" {
		ns = s.cfg.ManagedNamespace
	}
	return ns, s.namespaceAllowedForRequest(r, ns)
}

func (s *Server) namespacedQuery(r *http.Request) ([]string, error) {
	nsList, ok := s.namespacesFromQuery(r)
	if !ok {
		return nil, fmt.Errorf("namespace is not allowed")
	}
	return nsList, nil
}

func writeNamespacedList[T any](
	s *Server,
	w http.ResponseWriter,
	r *http.Request,
	resource string,
	fetch func(context.Context, string) ([]T, error),
	less func(a, b T) bool,
	postprocess ...func([]T) []T,
) {
	if !s.requirePermission(w, r, resource, "view") {
		return
	}

	nsList, err := s.namespacedQuery(r)
	if err != nil {
		writeErr(w, http.StatusForbidden, err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	chunks := make([][]T, len(nsList))
	var (
		wg       sync.WaitGroup
		firstErr error
		errOnce  sync.Once
	)
	type namespaceJob struct {
		index     int
		namespace string
	}
	jobs := make(chan namespaceJob, len(nsList))
	for i, ns := range nsList {
		jobs <- namespaceJob{index: i, namespace: ns}
	}
	close(jobs)
	workerCount := min(len(nsList), maxNamespaceListConcurrency)
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if ctx.Err() != nil {
					return
				}
				nsItems, err := fetch(ctx, job.namespace)
				if err != nil {
					errOnce.Do(func() {
						firstErr = err
						cancel()
					})
					return
				}
				chunks[job.index] = nsItems
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		writeErr(w, http.StatusInternalServerError, firstErr)
		return
	}

	items := make([]T, 0)
	for _, chunk := range chunks {
		items = append(items, chunk...)
	}
	if less != nil {
		sort.Slice(items, func(i, j int) bool { return less(items[i], items[j]) })
	}
	for _, fn := range postprocess {
		if fn != nil {
			items = fn(items)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func writeClusterList[T any](
	s *Server,
	w http.ResponseWriter,
	r *http.Request,
	resource string,
	fetch func(context.Context) ([]T, error),
) {
	if !s.requirePermission(w, r, resource, "view") {
		return
	}
	items, err := fetch(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
