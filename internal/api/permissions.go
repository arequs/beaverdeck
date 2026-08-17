package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"beaverdeck/internal/auth"
)

type rolePermission struct {
	View   bool `json:"view"`
	Edit   bool `json:"edit"`
	Delete bool `json:"delete"`
}

type rolePermissionSet struct {
	Namespaces []string                  `json:"namespaces"`
	Resources  map[string]rolePermission `json:"resources"`
}

func defaultPermissionSet() rolePermissionSet {
	return rolePermissionSet{Namespaces: []string{}, Resources: map[string]rolePermission{}}
}

func parsePermissionSet(raw json.RawMessage) rolePermissionSet {
	out := defaultPermissionSet()
	if len(raw) == 0 {
		return out
	}
	var parsed rolePermissionSet
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return out
	}
	if len(parsed.Namespaces) > 0 {
		seen := map[string]struct{}{}
		filtered := make([]string, 0, len(parsed.Namespaces))
		for _, ns := range parsed.Namespaces {
			n := strings.TrimSpace(ns)
			if n == "" {
				continue
			}
			if _, ok := seen[n]; ok {
				continue
			}
			seen[n] = struct{}{}
			filtered = append(filtered, n)
		}
		out.Namespaces = filtered
	}
	for k, v := range parsed.Resources {
		if v.Delete {
			v.Edit = true
			v.View = true
		}
		if v.Edit {
			v.View = true
		}
		if v.View || v.Edit || v.Delete {
			out.Resources[strings.ToLower(strings.TrimSpace(k))] = v
		}
	}
	return out
}

func (s *Server) namespacesFromQuery(r *http.Request) ([]string, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if raw == "" {
		return []string{s.cfg.ManagedNamespace}, s.namespaceAllowedForRequest(r, s.cfg.ManagedNamespace)
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		ns := strings.TrimSpace(part)
		if ns == "" {
			continue
		}
		if !s.namespaceAllowedForRequest(r, ns) {
			return nil, false
		}
		if _, ok := seen[ns]; ok {
			continue
		}
		seen[ns] = struct{}{}
		out = append(out, ns)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func (s *Server) namespaceAllowed(ns string) bool {
	if ns == "" {
		return false
	}
	if s.cfg.AllowAllNamespaces {
		return true
	}
	return ns == s.cfg.ManagedNamespace
}

func (s *Server) permissionSetFromRequest(r *http.Request) rolePermissionSet {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		return defaultPermissionSet()
	}
	return parsePermissionSet(u.Permissions)
}

func (s *Server) isNamespaceAllowedByRole(r *http.Request, ns string) bool {
	if ns == "" {
		return false
	}
	if auth.IsAdmin(r.Context()) {
		return true
	}
	perm := s.permissionSetFromRequest(r)
	if len(perm.Namespaces) == 0 {
		return true
	}
	for _, allowed := range perm.Namespaces {
		if ns == allowed {
			return true
		}
	}
	return false
}

func (s *Server) namespaceAllowedForRequest(r *http.Request, ns string) bool {
	if !s.namespaceAllowed(ns) {
		return false
	}
	return s.isNamespaceAllowedByRole(r, ns)
}

func (s *Server) hasPermission(r *http.Request, resource, action string) bool {
	if auth.IsAdmin(r.Context()) {
		return true
	}
	permSet := s.permissionSetFromRequest(r)
	resource = strings.ToLower(strings.TrimSpace(resource))
	perm, ok := permSet.Resources[resource]
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "view":
		return perm.View
	case "edit":
		return perm.Edit
	case "delete":
		return perm.Delete
	default:
		return false
	}
}

func (s *Server) requirePermission(w http.ResponseWriter, r *http.Request, resource, action string) bool {
	if s.hasPermission(r, resource, action) {
		return true
	}
	writeErr(w, http.StatusForbidden, fmt.Errorf("permission denied: %s %s", action, resource))
	return false
}

func kindToResource(kind string) string {
	if _, ok := customResourceCRDName(kind); ok {
		return "crds"
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "pod", "pods":
		return "pods"
	case "deployment", "deployments", "daemonset", "daemonsets", "statefulset", "statefulsets", "job", "jobs", "cronjob", "cronjobs", "replicaset", "replicasets", "replicationcontroller", "replicationcontrollers":
		return "workloads"
	case "node", "nodes":
		return "nodes"
	case "service", "services":
		return "services"
	case "serviceaccount", "serviceaccounts":
		return "serviceaccounts"
	case "ingress", "ingresses":
		return "ingresses"
	case "role", "roles", "rolebinding", "rolebindings":
		return "rbacroles"
	case "clusterrole", "clusterroles", "clusterrolebinding", "clusterrolebindings":
		return "clusterroles"
	case "configmap", "configmaps":
		return "configmaps"
	case "crd", "customresourcedefinition", "customresourcedefinitions":
		return "crds"
	case "secret", "secrets":
		return "secrets"
	case "pvc", "persistentvolumeclaim", "persistentvolumeclaims":
		return "pvcs"
	case "pv", "persistentvolume", "persistentvolumes":
		return "pvs"
	case "storageclass", "storageclasses":
		return "storageclasses"
	default:
		return ""
	}
}

const customResourceKindPrefix = "customresource:"

func customResourceCRDName(kind string) (string, bool) {
	normalized := strings.TrimSpace(kind)
	if len(normalized) <= len(customResourceKindPrefix) || !strings.EqualFold(normalized[:len(customResourceKindPrefix)], customResourceKindPrefix) {
		return "", false
	}
	name := strings.TrimSpace(normalized[len(customResourceKindPrefix):])
	return name, name != ""
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if auth.IsAdmin(r.Context()) {
		return true
	}
	writeErr(w, http.StatusForbidden, fmt.Errorf("admin role required"))
	return false
}

func stringsTrimOrFallback(primary, fallback string) string {
	value := strings.TrimSpace(primary)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}
