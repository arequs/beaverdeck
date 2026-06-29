package kube

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
)

func TestPodReferenceProblemsFindsRequiredMissingRefs(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "pull-secret"}},
			Volumes: []corev1.Volume{
				{
					Name: "required-secret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: "missing-secret"},
					},
				},
				{
					Name: "optional-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "missing-optional-config"},
							Optional:             ptr.To(true),
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name: "app",
					EnvFrom: []corev1.EnvFromSource{
						{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "missing-config"}}},
					},
				},
			},
		},
	}

	problems := podReferenceProblems(
		pod,
		map[string]struct{}{"pull-secret": {}},
		map[string]struct{}{},
	)

	if len(problems) != 2 {
		t.Fatalf("expected 2 missing required refs, got %d: %#v", len(problems), problems)
	}
	assertContains(t, problems, "volume required-secret references missing Secret missing-secret")
	assertContains(t, problems, "container app envFrom references missing ConfigMap missing-config")
}

func TestPodPrivilegeDetails(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			HostNetwork: true,
			Containers: []corev1.Container{
				{
					Name: "app",
					SecurityContext: &corev1.SecurityContext{
						Privileged:               ptr.To(true),
						AllowPrivilegeEscalation: ptr.To(true),
					},
				},
			},
		},
	}

	details := podPrivilegeDetails(pod)
	assertContains(t, details, "pod uses hostNetwork")
	assertContains(t, details, "container app is privileged")
	assertContains(t, details, "container app allows privilege escalation")
}

func TestSensitiveEnvNameUsesWildcardTokens(t *testing.T) {
	cases := map[string]bool{
		"PASSWORD":           true,
		"DB_PASSWORD":        true,
		"MY_TOKEN_VALUE":     true,
		"OIDC_CLIENT_SECRET": true,
		"PRIVATE_KEY_PEM":    true,
		"NORMAL_SETTING":     false,
	}
	for input, want := range cases {
		if got := sensitiveEnvName(input); got != want {
			t.Fatalf("sensitiveEnvName(%q)=%t, want %t", input, got, want)
		}
	}
}

func TestNamespaceGPUQuotaPresent(t *testing.T) {
	quotas := []corev1.ResourceQuota{
		{
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceName("requests.nvidia.com/gpu"): resource.MustParse("4"),
				},
			},
		},
	}

	if !namespaceGPUQuotaPresent(quotas) {
		t.Fatal("expected GPU quota to be detected")
	}
	if namespaceGPUQuotaPresent(nil) {
		t.Fatal("expected nil quotas to report no GPU quota")
	}
}

func TestGPUAllocationPressureInsightThresholds(t *testing.T) {
	warning := gpuAllocationPressureInsight("node", "gpu-a", 10, 8)
	if warning.Status != "alert" || warning.Severity != "warning" {
		t.Fatalf("expected warning alert, got status=%s severity=%s", warning.Status, warning.Severity)
	}

	critical := gpuAllocationPressureInsight("node", "gpu-a", 10, 10)
	if critical.Status != "alert" || critical.Severity != "critical" {
		t.Fatalf("expected critical alert, got status=%s severity=%s", critical.Status, critical.Severity)
	}

	ok := gpuAllocationPressureInsight("node", "gpu-a", 10, 2)
	if ok.Status != "ok" || ok.Severity != "ok" {
		t.Fatalf("expected ok status, got status=%s severity=%s", ok.Status, ok.Severity)
	}
}

func TestIngressRouteKeys(t *testing.T) {
	className := "nginx"
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "app"},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &className,
			Rules: []networkingv1.IngressRule{
				{
					Host: "app.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{Path: "/"}},
						},
					},
				},
			},
		},
	}

	keys := ingressRouteKeys(ingress)
	if got := keys["nginx|app.example.com|/"]; got != "class=nginx host=app.example.com path=/" {
		t.Fatalf("unexpected route label: %q", got)
	}
}

func TestNormalizeInsightCategoryNewSections(t *testing.T) {
	cases := map[string]string{
		"gpu":           "gpu",
		"accelerators":  "gpu",
		"security":      "security",
		"configuration": "configuration",
		"config":        "configuration",
	}
	for input, want := range cases {
		if got := normalizeInsightCategory(input); got != want {
			t.Fatalf("normalizeInsightCategory(%q)=%q, want %q", input, got, want)
		}
	}
	if got := normalizeInsightCategory("capacity"); got != unsupportedInsightCategory {
		t.Fatalf("normalizeInsightCategory(%q)=%q, want %q", "capacity", got, unsupportedInsightCategory)
	}
}

func TestSuppressedInsightsConfigMapAddedAndRemoved(t *testing.T) {
	ctx := context.Background()
	ref := SuppressedInsightsRef{Namespace: "beaverdeck", Name: "beaverdeck-suppressed-insights", Key: "suppressed_insights.json"}
	client := &Client{core: fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: ref.Namespace},
		Data:       map[string]string{ref.Key: "[]"},
	})}

	if err := client.SetInsightSuppressed(ctx, ref, "node:worker-1:pressure", true); err != nil {
		t.Fatal(err)
	}
	if err := client.SetInsightSuppressed(ctx, ref, "node:worker-1:pressure", true); err != nil {
		t.Fatal(err)
	}
	items, err := client.ListSuppressedInsights(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0] != "node:worker-1:pressure" {
		t.Fatalf("unexpected suppressed insights after add: %#v", items)
	}

	if err := client.SetInsightSuppressed(ctx, ref, "node:worker-1:pressure", false); err != nil {
		t.Fatal(err)
	}
	items, err = client.ListSuppressedInsights(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("unexpected suppressed insights after remove: %#v", items)
	}
}

func TestBuildInsightsFiltersCategorySpecificPodChecks(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mixed",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: ptr.To[int64](0),
			},
			Volumes: []corev1.Volume{
				{
					Name: "required-secret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: "missing-secret"},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx",
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To(true),
						RunAsUser:  ptr.To[int64](0),
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "app",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
		},
	}
	client := newInsightTestClient(pod)

	configurationAlerts, err := client.BuildInsights(context.Background(), []string{"default"}, "configuration")
	if err != nil {
		t.Fatal(err)
	}
	assertHasCheckType(t, configurationAlerts, "missing-references")
	assertMissingCheckType(t, configurationAlerts, "root-user")
	assertMissingCheckType(t, configurationAlerts, "container-waiting")

	securityAlerts, err := client.BuildInsights(context.Background(), []string{"default"}, "security")
	if err != nil {
		t.Fatal(err)
	}
	assertHasCheckType(t, securityAlerts, "root-user")
	assertHasCheckType(t, securityAlerts, "pod-privileged")
	assertMissingCheckType(t, securityAlerts, "missing-references")
	assertMissingCheckType(t, securityAlerts, "container-waiting")

	workloadAlerts, err := client.BuildInsights(context.Background(), []string{"default"}, "workloads")
	if err != nil {
		t.Fatal(err)
	}
	assertHasCheckType(t, workloadAlerts, "container-waiting")
	assertHasCheckType(t, workloadAlerts, "missing-requests")
	assertMissingCheckType(t, workloadAlerts, "root-user")
	assertMissingCheckType(t, workloadAlerts, "missing-references")
}

func TestBuildInsightsMovesCapacityChecksToWorkloadAndNodeCategories(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1"},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec: corev1.PodSpec{
			NodeName: "worker-1",
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	client := newInsightTestClient(node, pod)

	workloadAlerts, err := client.BuildInsights(context.Background(), []string{"default"}, "workloads")
	if err != nil {
		t.Fatal(err)
	}
	assertHasCheckType(t, workloadAlerts, "missing-requests")
	assertMissingCheckType(t, workloadAlerts, "node-capacity")
	assertMissingCheckType(t, workloadAlerts, "node-underutilized")

	nodeAlerts, err := client.BuildInsights(context.Background(), []string{"default"}, "nodes")
	if err != nil {
		t.Fatal(err)
	}
	assertHasCheckType(t, nodeAlerts, "node-capacity")
	assertHasCheckType(t, nodeAlerts, "node-underutilized")
	assertMissingCheckType(t, nodeAlerts, "missing-requests")
}

func TestBuildInsightsSecurityChecksNetworkPolicyAndSensitiveLiteralEnv(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx",
					Env: []corev1.EnvVar{
						{Name: "DB_PASSWORD", Value: "do-not-log"},
						{Name: "API_TOKEN", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "api-token"}, Key: "token"}}},
					},
				},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	client := newInsightTestClient(pod)

	alerts, err := client.BuildInsights(context.Background(), []string{"default"}, "security")
	if err != nil {
		t.Fatal(err)
	}
	assertHasCheckType(t, alerts, "sensitive-env-literal")
	assertHasCheckType(t, alerts, "network-policy-coverage")
	assertMissingDetail(t, alerts, "do-not-log")
}

func TestBuildInsightsGPUDetectsRequestedGPUWithoutGPUCapacity(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "gpu-app", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "cuda",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	client := newInsightTestClient(pod)

	alerts, err := client.BuildInsights(context.Background(), []string{"default"}, "gpu")
	if err != nil {
		t.Fatal(err)
	}
	assertHasCheckType(t, alerts, "gpu-capacity-discovery")
}

func TestBuildInsightsCategoryScopedResourceLoading(t *testing.T) {
	basePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "nginx"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cases := []struct {
		name       string
		category   string
		objects    []runtime.Object
		wantLists  []string
		blockLists []string
	}{
		{
			name:       "configuration",
			category:   "configuration",
			objects:    []runtime.Object{basePod},
			wantLists:  []string{"pods", "secrets", "configmaps"},
			blockLists: []string{"nodes", "events", "networkpolicies", "services", "ingresses", "deployments", "statefulsets", "daemonsets", "persistentvolumeclaims", "persistentvolumes", "resourcequotas"},
		},
		{
			name:       "security",
			category:   "security",
			objects:    []runtime.Object{basePod},
			wantLists:  []string{"pods", "networkpolicies"},
			blockLists: []string{"nodes", "events", "secrets", "configmaps", "services", "ingresses", "deployments", "statefulsets", "daemonsets", "persistentvolumeclaims", "persistentvolumes", "resourcequotas"},
		},
		{
			name:       "workloads",
			category:   "workloads",
			objects:    []runtime.Object{basePod},
			wantLists:  []string{"pods", "events", "daemonsets"},
			blockLists: []string{"nodes", "secrets", "configmaps", "networkpolicies", "services", "ingresses", "deployments", "statefulsets", "persistentvolumeclaims", "persistentvolumes", "resourcequotas"},
		},
		{
			name:       "networking",
			category:   "networking",
			objects:    nil,
			wantLists:  []string{"services", "endpointslices", "ingresses", "deployments", "statefulsets"},
			blockLists: []string{"nodes", "pods", "events", "secrets", "configmaps", "networkpolicies", "daemonsets", "persistentvolumeclaims", "persistentvolumes", "resourcequotas"},
		},
		{
			name:       "gpu",
			category:   "gpu",
			objects:    []runtime.Object{basePod},
			wantLists:  []string{"nodes", "pods", "events", "resourcequotas"},
			blockLists: []string{"secrets", "configmaps", "networkpolicies", "services", "ingresses", "deployments", "statefulsets", "daemonsets", "persistentvolumeclaims", "persistentvolumes"},
		},
		{
			name:       "nodes",
			category:   "nodes",
			objects:    []runtime.Object{basePod},
			wantLists:  []string{"nodes", "pods"},
			blockLists: []string{"events", "secrets", "configmaps", "networkpolicies", "services", "ingresses", "deployments", "statefulsets", "daemonsets", "persistentvolumeclaims", "persistentvolumes", "resourcequotas"},
		},
		{
			name:       "storage",
			category:   "storage",
			objects:    nil,
			wantLists:  []string{"nodes", "persistentvolumeclaims", "persistentvolumes"},
			blockLists: []string{"pods", "events", "secrets", "configmaps", "networkpolicies", "services", "ingresses", "deployments", "statefulsets", "daemonsets", "resourcequotas"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(tc.objects...)
			client := newInsightTestClientWithClientset(clientset)
			if _, err := client.BuildInsights(context.Background(), []string{"default"}, tc.category); err != nil {
				t.Fatal(err)
			}
			for _, resource := range tc.wantLists {
				if got := countListActions(clientset.Actions(), resource); got == 0 {
					t.Fatalf("expected %s insights to list %s, actions=%#v", tc.category, resource, clientset.Actions())
				}
			}
			for _, resource := range tc.blockLists {
				if got := countListActions(clientset.Actions(), resource); got != 0 {
					t.Fatalf("expected %s insights not to list %s, got %d actions=%#v", tc.category, resource, got, clientset.Actions())
				}
			}
		})
	}
}

func TestBuildInsightsNetworkingLoadsSecretsOnlyWhenTLSNeedsLookup(t *testing.T) {
	noTLSClientset := fake.NewSimpleClientset()
	noTLSClient := &Client{core: noTLSClientset}
	if _, err := noTLSClient.BuildInsights(context.Background(), []string{"default"}, "networking"); err != nil {
		t.Fatal(err)
	}
	if got := countListActions(noTLSClientset.Actions(), "secrets"); got != 0 {
		t.Fatalf("expected networking without TLS refs to skip secrets list, got %d", got)
	}

	tlsIngress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "app-tls"}},
		},
	}
	tlsClientset := fake.NewSimpleClientset(tlsIngress)
	tlsClient := &Client{core: tlsClientset}
	if _, err := tlsClient.BuildInsights(context.Background(), []string{"default"}, "networking"); err != nil {
		t.Fatal(err)
	}
	if got := countListActions(tlsClientset.Actions(), "secrets"); got != 1 {
		t.Fatalf("expected networking with TLS refs to list secrets once, got %d", got)
	}
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("expected %#v to contain %q", values, want)
}

func newInsightTestClient(objects ...runtime.Object) *Client {
	return newInsightTestClientWithClientset(fake.NewSimpleClientset(objects...))
}

func newInsightTestClientWithClientset(clientset *fake.Clientset) *Client {
	return &Client{
		core: clientset,
		metrics: resourceMetricsCache{
			podCPU:  make(map[string]resourceCounterSample),
			nodeCPU: make(map[string]resourceCounterSample),
		},
	}
}

func assertHasCheckType(t *testing.T, alerts []InsightAlert, checkType string) {
	t.Helper()
	for _, alert := range alerts {
		if alert.CheckType == checkType {
			return
		}
	}
	t.Fatalf("expected check type %q in alerts %#v", checkType, alerts)
}

func assertMissingCheckType(t *testing.T, alerts []InsightAlert, checkType string) {
	t.Helper()
	for _, alert := range alerts {
		if alert.CheckType == checkType {
			t.Fatalf("did not expect check type %q in alerts %#v", checkType, alerts)
		}
	}
}

func assertMissingDetail(t *testing.T, alerts []InsightAlert, value string) {
	t.Helper()
	for _, alert := range alerts {
		for _, detail := range alert.Details {
			if strings.Contains(detail, value) {
				t.Fatalf("did not expect detail to contain %q in alerts %#v", value, alerts)
			}
		}
	}
}

func countListActions(actions []k8stesting.Action, resource string) int {
	count := 0
	for _, action := range actions {
		if action.GetVerb() == "list" && action.GetResource().Resource == resource {
			count++
		}
	}
	return count
}
