package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"beaverdeck/internal/kube"
	"github.com/gorilla/websocket"
)

type scaleRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Replicas  int32  `json:"replicas"`
}

type restartRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type deletePodRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type deleteResourceRequest struct {
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
}

type nodeActionRequest struct {
	Name  string `json:"name"`
	Force bool   `json:"force"`
}

type applyRequest struct {
	Namespace    string `json:"namespace"`
	YAML         string `json:"yaml"`
	DryRun       bool   `json:"dryRun"`
	FieldManager string `json:"fieldManager"`
}

type updateManifestRequest struct {
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	YAML      string `json:"yaml"`
	DryRun    bool   `json:"dryRun"`
}

type execWSMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

func (s *Server) execWS(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "exec", "edit") {
		return
	}
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
	commands := r.URL.Query()["command"]
	useDefaultShell := len(commands) == 0

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	stdinReader, stdinWriter := io.Pipe()
	defer stdinReader.Close()
	defer stdinWriter.Close()
	wsWriter := &websocketWriter{conn: conn}
	resizeQueue := kube.NewTerminalResizeQueue()
	defer resizeQueue.Close()

	go func() {
		defer stdinWriter.Close()
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if len(msg) == 0 {
				continue
			}
			if msgType == websocket.BinaryMessage {
				var wsMsg execWSMessage
				if err := json.Unmarshal(msg, &wsMsg); err == nil {
					if wsMsg.Type == "resize" {
						resizeQueue.Push(wsMsg.Cols, wsMsg.Rows)
						continue
					}
				}
				continue
			}
			if len(msg) > 0 {
				_, _ = stdinWriter.Write(msg)
			}
		}
	}()

	var execErr error
	if useDefaultShell {
		execErr = s.kube.ExecDefaultShellWithSizeQueue(r.Context(), ns, pod, container, stdinReader, wsWriter, wsWriter, resizeQueue)
	} else {
		execErr = s.kube.ExecWithSizeQueue(r.Context(), ns, pod, container, commands, stdinReader, wsWriter, wsWriter, resizeQueue)
	}
	if execErr != nil {
		msg := strings.TrimSpace(execErr.Error())
		if strings.Contains(strings.ToLower(msg), "no interactive shell found in container") {
			msg = msg + "\r\nThis is normal for distroless or minimal images."
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[exec error] "+msg+"\r\n"))
	}
}

func (s *Server) scaleDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "workloads", "edit") {
		return
	}
	var req scaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !s.namespaceAllowedForRequest(r, req.Namespace) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	err := s.kube.ScaleDeployment(r.Context(), req.Namespace, req.Name, req.Replicas)
	s.logMutation(r.Context(), "scale", req.Namespace, "deployment", req.Name, false, err)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) scaleStatefulSet(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "workloads", "edit") {
		return
	}
	var req scaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !s.namespaceAllowedForRequest(r, req.Namespace) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	err := s.kube.ScaleStatefulSet(r.Context(), req.Namespace, req.Name, req.Replicas)
	s.logMutation(r.Context(), "scale", req.Namespace, "statefulset", req.Name, false, err)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) restartDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "workloads", "edit") {
		return
	}
	var req restartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !s.namespaceAllowedForRequest(r, req.Namespace) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	err := s.kube.RestartDeployment(r.Context(), req.Namespace, req.Name)
	s.logMutation(r.Context(), "restart", req.Namespace, "deployment", req.Name, false, err)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) deletePod(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "pods", "delete") {
		return
	}
	var req deletePodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !s.namespaceAllowedForRequest(r, req.Namespace) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	err := s.kube.DeletePod(r.Context(), req.Namespace, req.Name)
	s.logMutation(r.Context(), "delete", req.Namespace, "pod", req.Name, false, err)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) deleteResource(w http.ResponseWriter, r *http.Request) {
	var req deleteResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	resource, namespaced, err := permissionDeleteTarget(req.Kind)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !s.requirePermission(w, r, resource, "delete") {
		return
	}
	if crdName, custom := customResourceCRDName(req.Kind); custom {
		definition, resolveErr := s.kube.ResolveCustomResourceDefinition(r.Context(), crdName)
		if resolveErr != nil {
			writeErr(w, http.StatusInternalServerError, resolveErr)
			return
		}
		namespaced = definition.Namespaced
	}
	if namespaced && !s.namespaceAllowedForRequest(r, req.Namespace) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}

	err = s.kube.DeleteResource(r.Context(), req.Namespace, req.Kind, req.Name)
	s.logMutation(r.Context(), "delete", req.Namespace, strings.ToLower(strings.TrimSpace(req.Kind)), req.Name, false, err)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) evictPod(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "pods", "edit") {
		return
	}
	var req deletePodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !s.namespaceAllowedForRequest(r, req.Namespace) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	err := s.kube.EvictPod(r.Context(), req.Namespace, req.Name)
	s.logMutation(r.Context(), "evict", req.Namespace, "pod", req.Name, false, err)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func permissionDeleteTarget(kind string) (resource string, namespaced bool, err error) {
	if _, ok := customResourceCRDName(kind); ok {
		return "crds", false, nil
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "pod", "pods":
		return "pods", true, nil
	case "deployment", "deployments", "statefulset", "statefulsets", "daemonset", "daemonsets", "job", "jobs", "cronjob", "cronjobs":
		return "workloads", true, nil
	case "service", "services":
		return "services", true, nil
	case "ingress", "ingresses":
		return "ingresses", true, nil
	case "configmap", "configmaps":
		return "configmaps", true, nil
	case "secret", "secrets":
		return "secrets", true, nil
	case "serviceaccount", "serviceaccounts":
		return "serviceaccounts", true, nil
	case "role", "roles", "rolebinding", "rolebindings":
		return "rbacroles", true, nil
	case "clusterrole", "clusterroles", "clusterrolebinding", "clusterrolebindings":
		return "clusterroles", false, nil
	case "pvc", "persistentvolumeclaim", "persistentvolumeclaims":
		return "pvcs", true, nil
	case "pv", "persistentvolume", "persistentvolumes":
		return "pvs", false, nil
	case "storageclass", "storageclasses":
		return "storageclasses", false, nil
	case "crd", "customresourcedefinition", "customresourcedefinitions":
		return "crds", false, nil
	default:
		return "", false, fmt.Errorf("unsupported kind for delete: %s", kind)
	}
}

func (s *Server) drainNode(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "nodes", "edit") {
		return
	}
	var req nodeActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("node name is required"))
		return
	}
	result, err := s.kube.DrainNode(r.Context(), req.Name, req.Force)
	message := fmt.Sprintf("cordoned=%t force=%t evicted=%d skipped=%d failed=%d", result.Cordoned, req.Force, len(result.Evicted), len(result.Skipped), len(result.Failed))
	if err != nil && len(result.Failed) == 0 {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	status := "ok"
	if err != nil {
		status = "partial"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   status,
		"result":   result,
		"message":  message,
		"warning":  errString(err),
		"evicted":  len(result.Evicted),
		"skipped":  len(result.Skipped),
		"failed":   len(result.Failed),
		"force":    req.Force,
		"nodeName": req.Name,
	})
}

func (s *Server) uncordonNode(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "nodes", "edit") {
		return
	}
	var req nodeActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("node name is required"))
		return
	}
	err := s.kube.SetNodeUnschedulable(r.Context(), req.Name, false)
	s.logMutation(r.Context(), "uncordon", "", "node", req.Name, false, err)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "nodeName": req.Name})
}

func (s *Server) applyYAML(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, "apply", "edit") {
		return
	}
	var req applyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !s.namespaceAllowedForRequest(r, req.Namespace) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}
	results, err := s.kube.ApplyYAML(r.Context(), req.Namespace, req.YAML, req.FieldManager, req.DryRun)
	s.logMutation(r.Context(), "apply", req.Namespace, "manifest", "bulk", req.DryRun, err)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": results, "dryRun": req.DryRun})
}

func (s *Server) updateManifest(w http.ResponseWriter, r *http.Request) {
	var req updateManifestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Kind = strings.TrimSpace(req.Kind)
	req.Name = strings.TrimSpace(req.Name)
	if req.Kind == "" || req.Name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("kind and name are required"))
		return
	}
	resource := kindToResource(req.Kind)
	if resource == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("unsupported kind: %s", req.Kind))
		return
	}
	if !s.requirePermission(w, r, resource, "edit") {
		return
	}
	namespaced := true
	if crdName, custom := customResourceCRDName(req.Kind); custom {
		definition, resolveErr := s.kube.ResolveCustomResourceDefinition(r.Context(), crdName)
		if resolveErr != nil {
			writeErr(w, http.StatusInternalServerError, resolveErr)
			return
		}
		namespaced = definition.Namespaced
	}
	if namespaced && !s.namespaceAllowedForRequest(r, req.Namespace) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("namespace is not allowed"))
		return
	}

	result, err := s.kube.UpdateManifestYAML(
		r.Context(),
		req.Namespace,
		req.Kind,
		req.Name,
		req.YAML,
		req.DryRun,
	)
	s.logMutation(r.Context(), "update", req.Namespace, resource, req.Name, req.DryRun, err)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": []any{result}, "dryRun": req.DryRun})
}
