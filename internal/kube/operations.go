package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/yaml"
)

func (c *Client) ListClusterRoles(ctx context.Context) ([]ClusterRoleInfo, error) {
	items, err := c.core.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ClusterRoleInfo, 0, len(items.Items))
	for _, item := range items.Items {
		out = append(out, ClusterRoleInfo{
			Name:  item.Name,
			Rules: len(item.Rules),
			Age:   age(item.CreationTimestamp.Time),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ListClusterRoleBindings(ctx context.Context) ([]ClusterRoleBindingInfo, error) {
	items, err := c.core.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ClusterRoleBindingInfo, 0, len(items.Items))
	for _, item := range items.Items {
		out = append(out, ClusterRoleBindingInfo{
			Name:     item.Name,
			RoleRef:  fmt.Sprintf("%s/%s", item.RoleRef.Kind, item.RoleRef.Name),
			Subjects: summarizeRBACSubjects(item.Subjects),
			Age:      age(item.CreationTimestamp.Time),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ListRoles(ctx context.Context, ns string) ([]RoleInfo, error) {
	items, err := c.core.RbacV1().Roles(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]RoleInfo, 0, len(items.Items))
	for _, item := range items.Items {
		out = append(out, RoleInfo{
			Namespace: ns,
			Name:      item.Name,
			Rules:     len(item.Rules),
			Age:       age(item.CreationTimestamp.Time),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ListRoleBindings(ctx context.Context, ns string) ([]RoleBindingInfo, error) {
	items, err := c.core.RbacV1().RoleBindings(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]RoleBindingInfo, 0, len(items.Items))
	for _, item := range items.Items {
		out = append(out, RoleBindingInfo{
			Namespace: ns,
			Name:      item.Name,
			RoleRef:   fmt.Sprintf("%s/%s", item.RoleRef.Kind, item.RoleRef.Name),
			Subjects:  summarizeRBACSubjects(item.Subjects),
			Age:       age(item.CreationTimestamp.Time),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ListServiceAccounts(ctx context.Context, ns string) ([]ServiceAccountInfo, error) {
	items, err := c.core.CoreV1().ServiceAccounts(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ServiceAccountInfo, 0, len(items.Items))
	for _, item := range items.Items {
		out = append(out, ServiceAccountInfo{
			Namespace: ns,
			Name:      item.Name,
			Secrets:   len(item.Secrets),
			Age:       age(item.CreationTimestamp.Time),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) PodLogs(ctx context.Context, ns, pod, container string, tail int64) (string, error) {
	container, err := c.defaultPodContainer(ctx, ns, pod, container)
	if err != nil {
		return "", err
	}
	req := c.core.CoreV1().Pods(ns).GetLogs(pod, &corev1.PodLogOptions{Container: container, TailLines: &tail})
	b, err := req.DoRaw(ctx)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Client) FollowPodLogs(ctx context.Context, ns, pod, container string, tail int64) (io.ReadCloser, error) {
	container, err := c.defaultPodContainer(ctx, ns, pod, container)
	if err != nil {
		return nil, err
	}
	req := c.core.CoreV1().Pods(ns).GetLogs(pod, &corev1.PodLogOptions{Container: container, TailLines: &tail, Follow: true})
	return req.Stream(ctx)
}

func (c *Client) defaultPodContainer(ctx context.Context, ns, pod, container string) (string, error) {
	container = strings.TrimSpace(container)
	if container != "" {
		return container, nil
	}
	obj, err := c.core.CoreV1().Pods(ns).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if len(obj.Spec.Containers) == 0 {
		return "", nil
	}
	return obj.Spec.Containers[0].Name, nil
}

func (c *Client) WorkloadLogs(ctx context.Context, ns, kind, name string, tail int64) (string, error) {
	var pods []corev1.Pod
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "deployment":
		obj, err := c.core.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		pods, err = c.podsForSelector(ctx, ns, obj.Spec.Selector)
		if err != nil {
			return "", err
		}
	case "statefulset":
		obj, err := c.core.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		pods, err = c.podsForSelector(ctx, ns, obj.Spec.Selector)
		if err != nil {
			return "", err
		}
	case "daemonset":
		obj, err := c.core.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		pods, err = c.podsForSelector(ctx, ns, obj.Spec.Selector)
		if err != nil {
			return "", err
		}
	case "job":
		podList, err := c.core.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: map[string]string{"job-name": name}}),
		})
		if err != nil {
			return "", err
		}
		pods = podList.Items
	case "cronjob":
		jobs, err := c.core.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", err
		}
		for _, job := range jobs.Items {
			if !isOwnedBy(job.OwnerReferences, "CronJob", name) {
				continue
			}
			podList, err := c.core.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
				LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: map[string]string{"job-name": job.Name}}),
			})
			if err != nil {
				return "", err
			}
			pods = append(pods, podList.Items...)
		}
	default:
		return "", fmt.Errorf("unsupported workload kind: %s", kind)
	}
	if len(pods) == 0 {
		return fmt.Sprintf("No pods found for %s/%s in namespace %s", kind, name, ns), nil
	}

	sort.Slice(pods, func(i, j int) bool { return pods[i].Name < pods[j].Name })

	var b strings.Builder
	for _, pod := range pods {
		text, logErr := c.PodLogs(ctx, ns, pod.Name, "", tail)
		b.WriteString("===== ")
		b.WriteString(pod.Name)
		b.WriteString(" =====\n")
		if logErr != nil {
			b.WriteString("[log error] ")
			b.WriteString(logErr.Error())
			b.WriteString("\n\n")
			continue
		}
		b.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func (c *Client) podsForSelector(ctx context.Context, ns string, selector *metav1.LabelSelector) ([]corev1.Pod, error) {
	if selector == nil {
		return nil, fmt.Errorf("workload selector is empty")
	}
	podSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("invalid workload selector: %w", err)
	}
	pods, err := c.core.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: podSelector.String()})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func isOwnedBy(refs []metav1.OwnerReference, kind, name string) bool {
	for _, ref := range refs {
		if strings.EqualFold(ref.Kind, kind) && ref.Name == name {
			return true
		}
	}
	return false
}

func (c *Client) Exec(ctx context.Context, ns, pod, container string, command []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return c.ExecWithSizeQueue(ctx, ns, pod, container, command, stdin, stdout, stderr, nil)
}

func (c *Client) ExecWithSizeQueue(ctx context.Context, ns, pod, container string, command []string, stdin io.Reader, stdout, stderr io.Writer, sizeQueue remotecommand.TerminalSizeQueue) error {
	return c.execWithTTY(ctx, ns, pod, container, command, stdin, stdout, stderr, true, sizeQueue)
}

func (c *Client) execWithTTY(ctx context.Context, ns, pod, container string, command []string, stdin io.Reader, stdout, stderr io.Writer, tty bool, sizeQueue remotecommand.TerminalSizeQueue) error {
	container, err := c.defaultPodContainer(ctx, ns, pod, container)
	if err != nil {
		return err
	}
	req := c.core.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(ns).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: container,
		Command:   command,
		Stdin:     stdin != nil,
		Stdout:    true,
		Stderr:    true,
		TTY:       tty,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(c.rest, http.MethodPost, req.URL())
	if err != nil {
		return err
	}

	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             stdin,
		Stdout:            stdout,
		Stderr:            stderr,
		Tty:               tty,
		TerminalSizeQueue: sizeQueue,
	})
}

func (c *Client) ExecDefaultShell(ctx context.Context, ns, pod, container string, stdin io.Reader, stdout, stderr io.Writer) error {
	return c.ExecDefaultShellWithSizeQueue(ctx, ns, pod, container, stdin, stdout, stderr, nil)
}

func (c *Client) ExecDefaultShellWithSizeQueue(ctx context.Context, ns, pod, container string, stdin io.Reader, stdout, stderr io.Writer, sizeQueue remotecommand.TerminalSizeQueue) error {
	candidates := [][]string{
		{"/bin/bash", "-i"},
		{"/bin/sh", "-i"},
		{"/bin/ash", "-i"},
		{"/busybox/sh", "-i"},
		{"/busybox", "sh"},
		{"sh"},
	}

	var attempts []string
	for i, candidate := range candidates {
		err := c.execWithTTY(ctx, ns, pod, container, probeShellCommand(candidate), nil, io.Discard, io.Discard, false, nil)
		if looksLikeMissingExecBinary(err) {
			attempts = append(attempts, strings.Join(candidate, " "))
			if i+1 < len(candidates) {
				_, _ = io.WriteString(stderr, fmt.Sprintf("\r\n[exec] shell %q is unavailable, trying %q\r\n", strings.Join(candidate, " "), strings.Join(candidates[i+1], " ")))
			}
			continue
		}
		if err == nil {
			return c.ExecWithSizeQueue(ctx, ns, pod, container, candidate, stdin, stdout, stderr, sizeQueue)
		}
		return err
	}

	if len(attempts) == 0 {
		return fmt.Errorf("no interactive shell found in container")
	}
	return fmt.Errorf("no interactive shell found in container; tried: %s", strings.Join(attempts, ", "))
}

func probeShellCommand(candidate []string) []string {
	if len(candidate) == 0 {
		return []string{"sh", "-c", "exit 0"}
	}
	if len(candidate) >= 2 && candidate[0] == "/busybox" && candidate[1] == "sh" {
		return []string{"/busybox", "sh", "-c", "exit 0"}
	}
	return []string{candidate[0], "-c", "exit 0"}
}

func looksLikeMissingExecBinary(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "no such file or directory") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "stat /bin/") ||
		strings.Contains(msg, "exec: \"sh\"") ||
		strings.Contains(msg, "exit code 126") ||
		strings.Contains(msg, "exit code 127")
}

func (c *Client) ScaleDeployment(ctx context.Context, ns, name string, replicas int32) error {
	dep, err := c.core.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	dep.Spec.Replicas = &replicas
	_, err = c.core.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (c *Client) ScaleStatefulSet(ctx context.Context, ns, name string, replicas int32) error {
	sts, err := c.core.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	sts.Spec.Replicas = &replicas
	_, err = c.core.AppsV1().StatefulSets(ns).Update(ctx, sts, metav1.UpdateOptions{})
	return err
}

func (c *Client) RestartDeployment(ctx context.Context, ns, name string) error {
	dep, err := c.core.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = map[string]string{}
	}
	dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().UTC().Format(time.RFC3339)
	_, err = c.core.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (c *Client) DeletePod(ctx context.Context, ns, name string) error {
	return c.core.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *Client) DeleteResource(ctx context.Context, namespace, kind, name string) error {
	if crdName, ok := customResourceReference(kind); ok {
		definition, err := c.ResolveCustomResourceDefinition(ctx, crdName)
		if err != nil {
			return err
		}
		gvr := schema.GroupVersionResource{Group: definition.Group, Version: definition.Version, Resource: definition.Resource}
		if definition.Namespaced {
			if strings.TrimSpace(namespace) == "" {
				return fmt.Errorf("namespace is required for %s", definition.Kind)
			}
			return c.dyn.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		}
		return c.dyn.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
	}
	gvr, namespaced, err := deleteTargetForKind(kind)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("resource name is required")
	}
	if namespaced {
		namespace = strings.TrimSpace(namespace)
		if namespace == "" {
			return fmt.Errorf("namespace is required for %s", kind)
		}
		return c.dyn.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}
	return c.dyn.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *Client) EvictPod(ctx context.Context, ns, name string) error {
	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
	}
	return c.core.PolicyV1().Evictions(ns).Evict(ctx, eviction)
}

func deleteTargetForKind(kind string) (schema.GroupVersionResource, bool, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "pod", "pods":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}, true, nil
	case "deployment", "deployments":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, true, nil
	case "statefulset", "statefulsets":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, true, nil
	case "daemonset", "daemonsets":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, true, nil
	case "job", "jobs":
		return schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, true, nil
	case "cronjob", "cronjobs":
		return schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}, true, nil
	case "service", "services":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}, true, nil
	case "ingress", "ingresses":
		return schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, true, nil
	case "configmap", "configmaps":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, true, nil
	case "secret", "secrets":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, true, nil
	case "serviceaccount", "serviceaccounts":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"}, true, nil
	case "role", "roles":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}, true, nil
	case "rolebinding", "rolebindings":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}, true, nil
	case "clusterrole", "clusterroles":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}, false, nil
	case "clusterrolebinding", "clusterrolebindings":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"}, false, nil
	case "pvc", "persistentvolumeclaim", "persistentvolumeclaims":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}, true, nil
	case "pv", "persistentvolume", "persistentvolumes":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumes"}, false, nil
	case "storageclass", "storageclasses":
		return schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"}, false, nil
	case "crd", "customresourcedefinition", "customresourcedefinitions":
		return schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}, false, nil
	default:
		return schema.GroupVersionResource{}, false, fmt.Errorf("unsupported kind for delete: %s", kind)
	}
}

func (c *Client) SetNodeUnschedulable(ctx context.Context, name string, unschedulable bool) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, unschedulable))
	_, err := c.core.CoreV1().Nodes().Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func (c *Client) DrainNode(ctx context.Context, name string, force bool) (NodeDrainResult, error) {
	result := NodeDrainResult{
		Node:    name,
		Evicted: []string{},
		Skipped: []string{},
		Failed:  []string{},
	}

	node, err := c.core.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return result, err
	}
	if !node.Spec.Unschedulable {
		if err := c.SetNodeUnschedulable(ctx, name, true); err != nil {
			return result, err
		}
		result.Cordoned = true
	} else {
		result.Cordoned = true
	}

	pods, err := c.core.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: "spec.nodeName=" + name})
	if err != nil {
		return result, err
	}
	sort.Slice(pods.Items, func(i, j int) bool {
		if pods.Items[i].Namespace != pods.Items[j].Namespace {
			return pods.Items[i].Namespace < pods.Items[j].Namespace
		}
		return pods.Items[i].Name < pods.Items[j].Name
	})

	for _, pod := range pods.Items {
		podRef := pod.Namespace + "/" + pod.Name
		switch {
		case pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed:
			result.Skipped = append(result.Skipped, podRef+" (completed pod)")
			continue
		case pod.Annotations["kubernetes.io/config.mirror"] != "":
			result.Skipped = append(result.Skipped, podRef+" (mirror/static pod)")
			continue
		case podOwnedByKind(&pod, "DaemonSet"):
			result.Skipped = append(result.Skipped, podRef+" (DaemonSet-managed)")
			continue
		case !force && podHasUnsafeLocalStorage(&pod):
			result.Skipped = append(result.Skipped, podRef+" (local storage)")
			continue
		case !force && len(pod.OwnerReferences) == 0:
			result.Skipped = append(result.Skipped, podRef+" (unmanaged pod)")
			continue
		}

		eviction := &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace},
		}
		if err := c.core.PolicyV1().Evictions(pod.Namespace).Evict(ctx, eviction); err != nil {
			if apierrors.IsNotFound(err) {
				result.Skipped = append(result.Skipped, podRef+" (already gone)")
				continue
			}
			result.Failed = append(result.Failed, podRef+" ("+err.Error()+")")
			continue
		}
		result.Evicted = append(result.Evicted, podRef)
	}

	if len(result.Failed) > 0 {
		return result, fmt.Errorf("drain completed with %d failed evictions", len(result.Failed))
	}
	return result, nil
}

func podOwnedByKind(pod *corev1.Pod, kind string) bool {
	if pod == nil {
		return false
	}
	for _, owner := range pod.OwnerReferences {
		if strings.EqualFold(owner.Kind, kind) {
			return true
		}
	}
	return false
}

func podHasUnsafeLocalStorage(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, volume := range pod.Spec.Volumes {
		if volume.EmptyDir != nil || volume.HostPath != nil {
			return true
		}
	}
	return false
}

func (c *Client) ListEvents(ctx context.Context, ns string, limit int) ([]EventInfo, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	events, err := c.core.CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	items := events.Items
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastTimestamp.Time.After(items[j].LastTimestamp.Time)
	})
	if len(items) > limit {
		items = items[:limit]
	}

	out := make([]EventInfo, 0, len(items))
	for _, e := range items {
		obj := fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name)
		last := e.EventTime.Time
		if last.IsZero() {
			last = e.LastTimestamp.Time
		}
		out = append(out, EventInfo{
			Namespace: ns,
			ObjectUID: e.InvolvedObject.UID,
			Type:      e.Type,
			Reason:    e.Reason,
			Object:    obj,
			Message:   e.Message,
			Count:     e.Count,
			LastSeen:  last.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

func (c *Client) ApplyYAML(ctx context.Context, namespace, docs, fieldManager string, dryRun bool) ([]ApplyResult, error) {
	if fieldManager == "" {
		fieldManager = "beaverdeck"
	}
	decoder := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(docs), 4096)
	results := make([]ApplyResult, 0)

	for {
		var raw map[string]interface{}
		err := decoder.Decode(&raw)
		if err == io.EOF {
			break
		}
		if err != nil {
			return results, fmt.Errorf("decode yaml: %w", err)
		}
		if len(raw) == 0 {
			continue
		}

		obj := &unstructured.Unstructured{Object: raw}
		gvk := obj.GroupVersionKind()
		if gvk.Empty() {
			return results, fmt.Errorf("object missing apiVersion/kind")
		}
		if obj.GetName() == "" {
			return results, fmt.Errorf("object %s missing metadata.name", gvk.String())
		}

		mapping, err := c.mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
		if err != nil {
			c.discovery.Invalidate()
			mapping, err = c.mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
			if err != nil {
				return results, fmt.Errorf("rest mapping for %s: %w", gvk.String(), err)
			}
		}

		if mapping.Scope.Name() == meta.RESTScopeNameNamespace && obj.GetNamespace() == "" {
			obj.SetNamespace(namespace)
		}

		payload, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
		if err != nil {
			return results, fmt.Errorf("encode object: %w", err)
		}

		rcNS := c.dyn.Resource(mapping.Resource)
		var rc interface {
			Patch(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (*unstructured.Unstructured, error)
		} = rcNS
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			rc = rcNS.Namespace(obj.GetNamespace())
		}

		opts := metav1.PatchOptions{FieldManager: fieldManager, Force: boolPtr(true)}
		if dryRun {
			opts.DryRun = []string{metav1.DryRunAll}
		}

		applied, err := rc.Patch(ctx, obj.GetName(), types.ApplyPatchType, payload, opts)
		if err != nil {
			return results, fmt.Errorf("apply %s/%s: %w", obj.GetKind(), obj.GetName(), err)
		}

		results = append(results, ApplyResult{
			APIVersion: applied.GetAPIVersion(),
			Kind:       applied.GetKind(),
			Namespace:  applied.GetNamespace(),
			Name:       applied.GetName(),
		})
	}

	return results, nil
}

func (c *Client) UpdateManifestYAML(ctx context.Context, namespace, kind, name, doc string, dryRun bool) (ApplyResult, error) {
	obj, err := singleManifestObject(doc)
	if err != nil {
		return ApplyResult{}, err
	}

	gvk := obj.GroupVersionKind()
	if gvk.Empty() {
		return ApplyResult{}, fmt.Errorf("object missing apiVersion/kind")
	}
	customDefinition := CustomResourceDefinition{}
	if crdName, custom := customResourceReference(kind); custom {
		customDefinition, err = c.ResolveCustomResourceDefinition(ctx, crdName)
		if err != nil {
			return ApplyResult{}, err
		}
		if gvk.Group != customDefinition.Group || !strings.EqualFold(gvk.Kind, customDefinition.Kind) {
			return ApplyResult{}, fmt.Errorf("manifest %s/%s does not match CRD %s", gvk.Group, gvk.Kind, crdName)
		}
	} else if !strings.EqualFold(gvk.Kind, strings.TrimSpace(kind)) {
		return ApplyResult{}, fmt.Errorf("manifest kind %q does not match %q", gvk.Kind, kind)
	}
	if obj.GetName() != strings.TrimSpace(name) {
		return ApplyResult{}, fmt.Errorf("manifest name %q does not match %q", obj.GetName(), name)
	}

	mapping, err := c.mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
	if err != nil {
		c.discovery.Invalidate()
		mapping, err = c.mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
		if err != nil {
			return ApplyResult{}, fmt.Errorf("rest mapping for %s: %w", gvk.String(), err)
		}
	}
	if customDefinition.Name != "" && (mapping.Resource.Group != customDefinition.Group || mapping.Resource.Resource != customDefinition.Resource) {
		return ApplyResult{}, fmt.Errorf("manifest resource does not match CRD %s", customDefinition.Name)
	}

	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		if obj.GetNamespace() == "" {
			obj.SetNamespace(namespace)
		}
		if obj.GetNamespace() != namespace {
			return ApplyResult{}, fmt.Errorf("manifest namespace %q does not match %q", obj.GetNamespace(), namespace)
		}
	}

	rcNS := c.dyn.Resource(mapping.Resource)
	var rc interface {
		Update(context.Context, *unstructured.Unstructured, metav1.UpdateOptions, ...string) (*unstructured.Unstructured, error)
	} = rcNS
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		rc = rcNS.Namespace(obj.GetNamespace())
	}

	opts := metav1.UpdateOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}

	updated, err := rc.Update(ctx, obj, opts)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("update %s/%s: %w", obj.GetKind(), obj.GetName(), err)
	}

	return ApplyResult{
		APIVersion: updated.GetAPIVersion(),
		Kind:       updated.GetKind(),
		Namespace:  updated.GetNamespace(),
		Name:       updated.GetName(),
	}, nil
}

func singleManifestObject(doc string) (*unstructured.Unstructured, error) {
	decoder := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(doc), 4096)
	var obj *unstructured.Unstructured
	for {
		var raw map[string]interface{}
		err := decoder.Decode(&raw)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode yaml: %w", err)
		}
		if len(raw) == 0 {
			continue
		}
		if obj != nil {
			return nil, fmt.Errorf("edit manifest supports exactly one object")
		}
		obj = &unstructured.Unstructured{Object: raw}
	}
	if obj == nil {
		return nil, fmt.Errorf("manifest is empty")
	}
	return obj, nil
}

func (c *Client) GetManifestYAML(ctx context.Context, namespace, kind, name string) (string, error) {
	if crdName, ok := customResourceReference(kind); ok {
		definition, err := c.ResolveCustomResourceDefinition(ctx, crdName)
		if err != nil {
			return "", err
		}
		gvr := schema.GroupVersionResource{Group: definition.Group, Version: definition.Version, Resource: definition.Resource}
		var obj *unstructured.Unstructured
		if definition.Namespaced {
			obj, err = c.dyn.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		} else {
			obj, err = c.dyn.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		}
		if err != nil {
			return "", err
		}
		return objectToYAML(obj.Object)
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "pod":
		obj, err := c.core.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}))
	case "deployment":
		obj, err := c.core.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}))
	case "statefulset":
		obj, err := c.core.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}))
	case "daemonset":
		obj, err := c.core.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}))
	case "job":
		obj, err := c.core.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}))
	case "cronjob":
		obj, err := c.core.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"}))
	case "node":
		obj, err := c.core.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Node"}))
	case "ingress":
		obj, err := c.core.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"}))
	case "secret":
		obj, err := c.core.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}))
	case "configmap":
		obj, err := c.core.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}))
	case "service":
		obj, err := c.core.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}))
	case "serviceaccount":
		obj, err := c.core.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ServiceAccount"}))
	case "role":
		obj, err := c.core.RbacV1().Roles(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"}))
	case "rolebinding":
		obj, err := c.core.RbacV1().RoleBindings(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"}))
	case "clusterrole":
		obj, err := c.core.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}))
	case "clusterrolebinding":
		obj, err := c.core.RbacV1().ClusterRoleBindings().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"}))
	case "pvc", "persistentvolumeclaim":
		obj, err := c.core.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"}))
	case "pv", "persistentvolume":
		obj, err := c.core.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolume"}))
	case "storageclass":
		obj, err := c.core.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(ensureObjectGVK(obj, schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"}))
	case "crd", "customresourcedefinition":
		obj, err := c.dyn.Resource(schema.GroupVersionResource{
			Group:    "apiextensions.k8s.io",
			Version:  "v1",
			Resource: "customresourcedefinitions",
		}).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return objectToYAML(obj.Object)
	default:
		return "", fmt.Errorf("unsupported kind: %s", kind)
	}
}

func customResourceReference(kind string) (string, bool) {
	const prefix = "customresource:"
	normalized := strings.TrimSpace(kind)
	if len(normalized) <= len(prefix) || !strings.EqualFold(normalized[:len(prefix)], prefix) {
		return "", false
	}
	name := strings.TrimSpace(normalized[len(prefix):])
	return name, name != ""
}

func (c *Client) GetSecretDecodedData(ctx context.Context, namespace, name string) (map[string]string, error) {
	secret, err := c.core.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(secret.Data))
	for key, value := range secret.Data {
		out[key] = string(value)
	}
	return out, nil
}

func objectToYAML(obj any) (string, error) {
	j, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	var payload any
	if err := json.Unmarshal(j, &payload); err != nil {
		return "", err
	}
	payload = stripManagedFields(payload)

	cleanJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	y, err := yaml.JSONToYAML(cleanJSON)
	if err != nil {
		return "", err
	}
	return string(y), nil
}

func ensureObjectGVK(obj runtime.Object, gvk schema.GroupVersionKind) runtime.Object {
	if obj == nil {
		return obj
	}
	copy := obj.DeepCopyObject()
	copy.GetObjectKind().SetGroupVersionKind(gvk)
	return copy
}

func stripManagedFields(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if k == "managedFields" {
				continue
			}
			out[k] = stripManagedFields(val)
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			out = append(out, stripManagedFields(item))
		}
		return out
	default:
		return t
	}
}
