package api

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"beaverdeck/internal/config"
	"beaverdeck/internal/kube"
	"beaverdeck/internal/users"
)

type Server struct {
	cfg   config.Config
	kube  *kube.Client
	users *users.Store
	fs    http.FileSystem
}

func New(cfg config.Config, kc *kube.Client, userStore *users.Store, webFS embed.FS) *Server {
	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		sub = webFS
	}
	return &Server{cfg: cfg, kube: kc, users: userStore, fs: http.FS(sub)}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/auth/providers", s.authProviders)
	mux.HandleFunc("GET /api/auth/bootstrap/status", s.authBootstrapStatus)
	mux.HandleFunc("POST /api/auth/bootstrap/complete", s.authBootstrapComplete)
	mux.HandleFunc("POST /api/auth/login", s.authLogin)
	mux.HandleFunc("POST /api/auth/logout", s.authLogout)
	mux.HandleFunc("GET /api/auth/google/start", s.authGoogleStart)
	mux.HandleFunc("GET /api/auth/google/callback", s.authGoogleCallback)
	mux.HandleFunc("GET /api/auth/oidc/start", s.authOIDCStart)
	mux.HandleFunc("GET /api/auth/oidc/callback", s.authOIDCCallback)

	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/me", s.me)
	mux.HandleFunc("GET /api/admin/users", s.adminUsersList)
	mux.HandleFunc("POST /api/admin/users", s.adminUsersCreate)
	mux.HandleFunc("POST /api/admin/users/role", s.adminUsersUpdateRole)
	mux.HandleFunc("POST /api/admin/users/password-reset", s.adminUsersResetPassword)
	mux.HandleFunc("POST /api/admin/users/delete", s.adminUsersDelete)
	mux.HandleFunc("GET /api/admin/roles", s.adminRolesList)
	mux.HandleFunc("POST /api/admin/roles", s.adminRolesCreate)
	mux.HandleFunc("POST /api/admin/roles/update", s.adminRolesUpdate)
	mux.HandleFunc("POST /api/admin/roles/delete", s.adminRolesDelete)
	mux.HandleFunc("GET /api/admin/google/config", s.adminGoogleConfigGet)
	mux.HandleFunc("POST /api/admin/google/config", s.adminGoogleConfigUpdate)
	mux.HandleFunc("POST /api/admin/google/config/test", s.adminGoogleConfigTest)
	mux.HandleFunc("POST /api/admin/google/reset", s.adminGoogleReset)
	mux.HandleFunc("GET /api/admin/google/mappings", s.adminGoogleMappingsList)
	mux.HandleFunc("POST /api/admin/google/mappings", s.adminGoogleMappingsUpsert)
	mux.HandleFunc("POST /api/admin/google/mappings/delete", s.adminGoogleMappingsDelete)
	mux.HandleFunc("GET /api/admin/oidc/config", s.adminOIDCConfigGet)
	mux.HandleFunc("POST /api/admin/oidc/config", s.adminOIDCConfigUpdate)
	mux.HandleFunc("POST /api/admin/oidc/config/test", s.adminOIDCConfigTest)
	mux.HandleFunc("POST /api/admin/oidc/reset", s.adminOIDCReset)
	mux.HandleFunc("GET /api/admin/oidc/mappings", s.adminOIDCMappingsList)
	mux.HandleFunc("POST /api/admin/oidc/mappings", s.adminOIDCMappingsUpsert)
	mux.HandleFunc("POST /api/admin/oidc/mappings/delete", s.adminOIDCMappingsDelete)
	mux.HandleFunc("GET /api/admin/config/export", s.adminConfigExport)
	mux.HandleFunc("POST /api/admin/config/import", s.adminConfigImport)

	mux.HandleFunc("GET /api/namespaces", s.namespaces)
	mux.HandleFunc("GET /api/workloads", s.workloads)
	mux.HandleFunc("GET /api/pods", s.pods)
	mux.HandleFunc("GET /api/nodes", s.nodes)
	mux.HandleFunc("GET /api/ingresses", s.ingresses)
	mux.HandleFunc("GET /api/secrets", s.secrets)
	mux.HandleFunc("GET /api/configmaps", s.configMaps)
	mux.HandleFunc("GET /api/crds", s.crds)
	mux.HandleFunc("GET /api/crds/{name}/resources", s.customResources)
	mux.HandleFunc("GET /api/services", s.services)
	mux.HandleFunc("GET /api/clusterroles", s.clusterRoles)
	mux.HandleFunc("GET /api/clusterrolebindings", s.clusterRoleBindings)
	mux.HandleFunc("GET /api/rbac/roles", s.rbacRoles)
	mux.HandleFunc("GET /api/rbac/rolebindings", s.rbacRoleBindings)
	mux.HandleFunc("GET /api/serviceaccounts", s.serviceAccounts)
	mux.HandleFunc("GET /api/pvcs", s.pvcs)
	mux.HandleFunc("GET /api/pvs", s.pvs)
	mux.HandleFunc("GET /api/storageclasses", s.storageClasses)
	mux.HandleFunc("GET /api/events", s.events)
	mux.HandleFunc("GET /api/insights", s.insights)
	mux.HandleFunc("POST /api/insights/suppress", s.setInsightSuppressed)
	mux.HandleFunc("GET /api/manifest", s.manifest)
	mux.HandleFunc("PUT /api/manifest", s.updateManifest)
	mux.HandleFunc("GET /api/podlogs", s.podLogs)
	mux.HandleFunc("GET /api/workloadlogs", s.workloadLogs)
	mux.HandleFunc("GET /api/pods/exec/ws", s.execWS)
	mux.HandleFunc("POST /api/nodes/drain", s.drainNode)
	mux.HandleFunc("POST /api/nodes/uncordon", s.uncordonNode)
	mux.HandleFunc("POST /api/deployments/scale", s.scaleDeployment)
	mux.HandleFunc("POST /api/statefulsets/scale", s.scaleStatefulSet)
	mux.HandleFunc("POST /api/deployments/restart", s.restartDeployment)
	mux.HandleFunc("POST /api/pods/evict", s.evictPod)
	mux.HandleFunc("POST /api/pods/delete", s.deletePod)
	mux.HandleFunc("POST /api/resources/delete", s.deleteResource)
	mux.HandleFunc("POST /api/apply", s.applyYAML)

	fileServer := http.FileServer(s.fs)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if !strings.Contains(filepath.Base(r.URL.Path), ".") {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}))

	return mux
}
