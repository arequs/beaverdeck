package api

import (
	"net/http"

	"beaverdeck/internal/users"
)

const adminConfigImportMaxBytes = 2 * 1024 * 1024

func (s *Server) adminConfigExport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	snapshot, err := s.users.ExportConfig(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	data, err := users.EncodeConfigSnapshot(snapshot)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="beaverdeck-config.yaml"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) adminConfigImport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, adminConfigImportMaxBytes)
	snapshot, err := users.DecodeConfigSnapshot(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	normalized, err := users.NormalizeConfigSnapshot(snapshot)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.users.ReplaceConfigSnapshot(r.Context(), normalized); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"users":           len(normalized.Users),
		"roles":           len(normalized.Roles),
		"google_mappings": len(normalized.Google.Mappings),
		"oidc_mappings":   len(normalized.OIDC.Mappings),
		"entra_mappings":  len(normalized.Entra.Mappings),
	})
}
