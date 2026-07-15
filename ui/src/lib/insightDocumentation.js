const INSIGHT_DOCUMENTATION_BASE_URL = 'https://beaverdeck.io/docs/insights-guide';

export const INSIGHT_DOCUMENTATION_PATHS = Object.freeze({
  'container-waiting': 'workloads/container-waiting',
  'daemonset-readiness': 'workloads/daemonset-readiness',
  'gpu-allocation-pressure': 'gpu/allocation-pressure',
  'gpu-capacity-discovery': 'gpu/capacity-discovery',
  'gpu-fragmentation': 'gpu/fragmentation',
  'gpu-namespace-usage': 'gpu/namespace-usage',
  'gpu-node-cordoned': 'gpu/node-scheduling',
  'gpu-node-idle-allocation': 'gpu/idle-allocation',
  'gpu-pod-pending': 'gpu/pod-pending',
  'gpu-pod-requests': 'gpu/pod-requests',
  'gpu-pod-unready': 'gpu/pod-readiness',
  'gpu-quota': 'gpu/quota',
  'ingress-backend': 'networking/ingress-backend',
  'ingress-route-collision': 'networking/route-collision',
  'ingress-tls': 'networking/ingress-tls',
  'loadbalancer-pending': 'networking/loadbalancer',
  'metrics-pipeline': 'nodes/metrics-pipeline',
  'missing-references': 'configuration/missing-references',
  'missing-requests': 'workloads/resource-requests',
  'network-policy-coverage': 'security/network-policy-coverage',
  'node-capacity': 'nodes/node-capacity',
  'node-condition': 'nodes/node-conditions',
  'node-underutilized': 'nodes/node-underutilization',
  'non-gpu-pods-on-gpu-node': 'gpu/workload-mix',
  oomkilled: 'workloads/oomkilled',
  'pod-pending': 'workloads/pod-pending',
  'pod-privileged': 'security/pod-privileges',
  'pod-request-usage': 'workloads/pod-request-usage',
  'pod-restarts': 'workloads/pod-restarts',
  'pod-under-request': 'workloads/overrequested-pod',
  'pv-released': 'storage/released-pv',
  'pvc-binding': 'storage/pvc-binding',
  'pvc-usage': 'storage/pvc-usage',
  'root-user': 'security/root-user',
  'sensitive-env-literal': 'security/sensitive-env-vars',
  'service-coverage': 'networking/service-coverage',
  'service-endpoints': 'networking/service-endpoints'
});

export function getInsightDocumentationUrl(checkType) {
  const path = INSIGHT_DOCUMENTATION_PATHS[String(checkType || '').trim()];
  return path ? `${INSIGHT_DOCUMENTATION_BASE_URL}/${path}/` : '';
}
