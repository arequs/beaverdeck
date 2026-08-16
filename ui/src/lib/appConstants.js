export const MENU = [
  {
    section: 'Workloads',
    items: [
      { id: 'pods', label: 'Pods' },
      { id: 'workloads', label: 'Workloads' },
      { id: 'nodes', label: 'Nodes' },
      { id: 'events', label: 'Events' }
    ]
  },
  {
    section: 'Networking',
    items: [
      { id: 'services', label: 'Services' },
      { id: 'ingresses', label: 'Ingresses' }
    ]
  },
  {
    section: 'RBAC',
    items: [
      { id: 'clusterroles', label: 'Cluster Roles' },
      { id: 'rbacroles', label: 'Roles' },
      { id: 'serviceaccounts', label: 'Service Accounts' }
    ]
  },
  {
    section: 'Config',
    items: [
      { id: 'configmaps', label: 'ConfigMaps' },
      { id: 'crds', label: 'CRDs' },
      { id: 'secrets', label: 'Secrets' }
    ]
  },
  {
    section: 'Storage',
    items: [
      { id: 'pvcs', label: 'PVC' },
      { id: 'pvs', label: 'PV' },
      { id: 'storageclasses', label: 'StorageClasses' }
    ]
  },
  {
    section: 'Insights',
    items: [
      { id: 'insights-nodes', label: 'Node Insights' },
      { id: 'insights-workloads', label: 'Workload Insights' },
      { id: 'insights-gpu', label: 'GPU Insights' },
      { id: 'insights-networking', label: 'Network Insights' },
      { id: 'insights-storage', label: 'Storage Insights' },
      { id: 'insights-security', label: 'Security Insights' },
      { id: 'insights-configuration', label: 'Configuration Insights' },
      { id: 'cluster-health', label: 'Cluster Health' }
    ]
  },
  {
    section: 'Admin',
    items: [
      { id: 'user-management', label: 'User Management' },
      { id: 'apply', label: 'Apply YAML' }
    ]
  }
];

export const NAMESPACE_STORAGE_KEY = 'beaverdeck-selected-namespaces';
export const THEME_STORAGE_PREFIX = 'beaverdeck-theme:';
export const THEME_OPTIONS = [
  { value: 'auto', label: 'System' },
  { value: 'dark', label: 'Dark' },
  { value: 'light', label: 'Light' }
];

export const INSIGHT_NAV_CATEGORIES = {
  insights: { value: 'nodes', label: 'Nodes' },
  'insights-nodes': { value: 'nodes', label: 'Nodes' },
  'insights-workloads': { value: 'workloads', label: 'Workloads' },
  'insights-gpu': { value: 'gpu', label: 'GPU' },
  'insights-networking': { value: 'networking', label: 'Network' },
  'insights-storage': { value: 'storage', label: 'Storage' },
  'insights-security': { value: 'security', label: 'Security' },
  'insights-configuration': { value: 'configuration', label: 'Configuration' }
};
export const INSIGHT_NAV_IDS = Object.keys(INSIGHT_NAV_CATEGORIES);

export const SORT_DEFAULTS = {
  pods: { key: 'name', dir: 'asc' },
  workloads: { key: 'name', dir: 'asc' },
  nodes: { key: 'name', dir: 'asc' },
  events: { key: 'object', dir: 'asc' },
  insights: { key: 'severity', dir: 'asc' },
  services: { key: 'name', dir: 'asc' },
  clusterroles: { key: 'name', dir: 'asc' },
  rbacroles: { key: 'name', dir: 'asc' },
  serviceaccounts: { key: 'name', dir: 'asc' },
  ingresses: { key: 'name', dir: 'asc' },
  configmaps: { key: 'name', dir: 'asc' },
  crds: { key: 'name', dir: 'asc' },
  customresources: { key: 'name', dir: 'asc' },
  secrets: { key: 'name', dir: 'asc' },
  pvcs: { key: 'name', dir: 'asc' },
  pvs: { key: 'name', dir: 'asc' },
  storageclasses: { key: 'name', dir: 'asc' }
};

export const ROLE_RESOURCES = ['pods', 'workloads', 'nodes', 'services', 'clusterroles', 'rbacroles', 'serviceaccounts', 'ingresses', 'configmaps', 'crds', 'secrets', 'pvcs', 'pvs', 'storageclasses', 'events', 'insights', 'exec', 'apply'];
export const CLUSTER_SCOPED_RESOURCES = new Set(['nodes', 'pvs', 'storageclasses']);
export const BOTTOM_DOCK_HIDDEN_NAVS = new Set([...INSIGHT_NAV_IDS, 'cluster-health', 'user-management', 'apply']);
export const DOCK_TOP_RATIO_DEFAULT = 0.62;
export const DOCK_TOP_RATIO_MIN = 0.3;
export const DOCK_TOP_RATIO_MAX = 0.8;

export const NAV_RESOURCE = {
  pods: 'pods',
  workloads: 'workloads',
  nodes: 'nodes',
  events: 'events',
  ...Object.fromEntries(INSIGHT_NAV_IDS.map((id) => [id, 'insights'])),
  services: 'services',
  clusterroles: 'clusterroles',
  rbacroles: 'rbacroles',
  serviceaccounts: 'serviceaccounts',
  ingresses: 'ingresses',
  configmaps: 'configmaps',
  crds: 'crds',
  secrets: 'secrets',
  pvcs: 'pvcs',
  pvs: 'pvs',
  storageclasses: 'storageclasses',
  apply: 'apply'
};

export const ROLE_RESOURCE_OPTIONS = {
  pods: [
    { value: 'none', label: 'No access', hint: 'Cannot view or manage pods.' },
    { value: 'view', label: 'View pods', hint: 'Can view pod lists, manifests and logs.' },
    { value: 'edit', label: 'Manage pods', hint: 'Can edit pod manifests and evict pods.' },
    { value: 'full', label: 'Full pod access', hint: 'Can view, edit and delete pods.' }
  ],
  workloads: [
    { value: 'none', label: 'No access', hint: 'Cannot view or manage workloads.' },
    { value: 'view', label: 'View workloads', hint: 'Can view workload lists, manifests and logs.' },
    { value: 'edit', label: 'Manage workloads', hint: 'Can scale, restart and edit workloads.' },
    { value: 'full', label: 'Full workload access', hint: 'Can view, edit and delete workloads where supported.' }
  ],
  nodes: [
    { value: 'none', label: 'No access', hint: 'Cannot view nodes.' },
    { value: 'view', label: 'View nodes', hint: 'Can view node list and manifests.' },
    { value: 'edit', label: 'Manage nodes', hint: 'Can edit node manifests and operate drain/uncordon actions.' }
  ],
  services: [
    { value: 'none', label: 'No access', hint: 'Cannot view services.' },
    { value: 'view', label: 'View services', hint: 'Can view service list and manifests.' },
    { value: 'edit', label: 'Manage services', hint: 'Can edit services.' },
    { value: 'full', label: 'Full service access', hint: 'Can view, edit and delete services.' }
  ],
  clusterroles: [
    { value: 'none', label: 'No access', hint: 'Cannot view ClusterRoles or ClusterRoleBindings.' },
    { value: 'view', label: 'View RBAC cluster', hint: 'Can view ClusterRoles and ClusterRoleBindings.' },
    { value: 'edit', label: 'Manage RBAC cluster', hint: 'Can edit ClusterRoles and ClusterRoleBindings.' },
    { value: 'full', label: 'Full RBAC cluster access', hint: 'Can view, edit and delete ClusterRoles and ClusterRoleBindings.' }
  ],
  rbacroles: [
    { value: 'none', label: 'No access', hint: 'Cannot view Roles or RoleBindings.' },
    { value: 'view', label: 'View RBAC roles', hint: 'Can view Roles and RoleBindings.' },
    { value: 'edit', label: 'Manage RBAC roles', hint: 'Can edit Roles and RoleBindings.' },
    { value: 'full', label: 'Full RBAC role access', hint: 'Can view, edit and delete Roles and RoleBindings.' }
  ],
  serviceaccounts: [
    { value: 'none', label: 'No access', hint: 'Cannot view ServiceAccounts.' },
    { value: 'view', label: 'View ServiceAccounts', hint: 'Can view ServiceAccounts and manifests.' },
    { value: 'edit', label: 'Manage ServiceAccounts', hint: 'Can edit ServiceAccounts.' },
    { value: 'full', label: 'Full ServiceAccount access', hint: 'Can view, edit and delete ServiceAccounts.' }
  ],
  ingresses: [
    { value: 'none', label: 'No access', hint: 'Cannot view ingresses.' },
    { value: 'view', label: 'View ingresses', hint: 'Can view ingress list and manifests.' },
    { value: 'edit', label: 'Manage ingresses', hint: 'Can edit ingresses.' },
    { value: 'full', label: 'Full ingress access', hint: 'Can view, edit and delete ingresses.' }
  ],
  configmaps: [
    { value: 'none', label: 'No access', hint: 'Cannot view ConfigMaps.' },
    { value: 'view', label: 'View ConfigMaps', hint: 'Can view ConfigMaps and manifests.' },
    { value: 'edit', label: 'Manage ConfigMaps', hint: 'Can edit ConfigMaps.' },
    { value: 'full', label: 'Full ConfigMap access', hint: 'Can view, edit and delete ConfigMaps.' }
  ],
  crds: [
    { value: 'none', label: 'No access', hint: 'Cannot view CustomResourceDefinitions.' },
    { value: 'view', label: 'View CRDs', hint: 'Can view CustomResourceDefinitions and custom resources in allowed namespaces.' },
    { value: 'edit', label: 'Manage CRDs', hint: 'Can edit CustomResourceDefinitions and custom resources in allowed namespaces.' },
    { value: 'full', label: 'Full CRD access', hint: 'Can view, edit and delete CustomResourceDefinitions and custom resources in allowed namespaces.' }
  ],
  secrets: [
    { value: 'none', label: 'No access', hint: 'Cannot view Secrets.' },
    { value: 'view', label: 'List Secrets', hint: 'Can list Secret metadata only; cannot open manifests or data.' },
    { value: 'edit', label: 'Reveal and manage Secrets', hint: 'Can open Secret manifests, reveal decoded data and edit manifests.' },
    { value: 'full', label: 'Full secret access', hint: 'Can reveal, edit and delete Secrets.' }
  ],
  pvcs: [
    { value: 'none', label: 'No access', hint: 'Cannot view PVCs.' },
    { value: 'view', label: 'View PVCs', hint: 'Can view PVC list and manifests.' },
    { value: 'edit', label: 'Manage PVCs', hint: 'Can edit PVCs.' },
    { value: 'full', label: 'Full PVC access', hint: 'Can view, edit and delete PVCs.' }
  ],
  pvs: [
    { value: 'none', label: 'No access', hint: 'Cannot view PVs.' },
    { value: 'view', label: 'View PVs', hint: 'Can view PV list and manifests.' },
    { value: 'edit', label: 'Manage PVs', hint: 'Can edit PVs.' },
    { value: 'full', label: 'Full PV access', hint: 'Can view, edit and delete PVs.' }
  ],
  storageclasses: [
    { value: 'none', label: 'No access', hint: 'Cannot view StorageClasses.' },
    { value: 'view', label: 'View StorageClasses', hint: 'Can view StorageClasses and manifests.' },
    { value: 'edit', label: 'Manage StorageClasses', hint: 'Can edit StorageClasses.' },
    { value: 'full', label: 'Full StorageClass access', hint: 'Can view, edit and delete StorageClasses.' }
  ],
  events: [
    { value: 'none', label: 'No access', hint: 'Cannot view events.' },
    { value: 'view', label: 'View events', hint: 'Can view cluster and namespace events.' }
  ],
  insights: [
    { value: 'none', label: 'No access', hint: 'Cannot open Insights.' },
    { value: 'view', label: 'View insights', hint: 'Can see alerts and dashboards.' },
    { value: 'edit', label: 'Manage insights', hint: 'Can suppress and restore alerts.' }
  ],
  exec: [
    { value: 'none', label: 'No access', hint: 'Cannot use pod exec.' },
    { value: 'edit', label: 'Use exec', hint: 'Can open exec sessions in running pods when View pods is also granted.' }
  ],
  apply: [
    { value: 'none', label: 'No access', hint: 'Cannot use Apply YAML.' },
    { value: 'edit', label: 'Apply YAML', hint: 'Can dry-run and apply manifests.' }
  ]
};

export const APPLY_TEMPLATES = {
  'ConfigMap': `apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sample-config\n  namespace: default\ndata:\n  APP_ENV: dev\n  LOG_LEVEL: info\n`,
  'CRD (Sample Widget)': `apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: widgets.demo.beaverdeck.io\nspec:\n  group: demo.beaverdeck.io\n  scope: Namespaced\n  names:\n    plural: widgets\n    singular: widget\n    kind: Widget\n    shortNames:\n      - wdgt\n  versions:\n    - name: v1alpha1\n      served: true\n      storage: true\n      schema:\n        openAPIV3Schema:\n          type: object\n          properties:\n            spec:\n              type: object\n              required:\n                - message\n              properties:\n                message:\n                  type: string\n                replicas:\n                  type: integer\n                  minimum: 1\n                  default: 1\n                enabled:\n                  type: boolean\n                  default: true\n      additionalPrinterColumns:\n        - name: Message\n          type: string\n          jsonPath: .spec.message\n        - name: Replicas\n          type: integer\n          jsonPath: .spec.replicas\n        - name: Enabled\n          type: boolean\n          jsonPath: .spec.enabled\n`,
  'Custom Resource (Sample Widget)': `apiVersion: demo.beaverdeck.io/v1alpha1\nkind: Widget\nmetadata:\n  name: sample-widget\nspec:\n  message: Hello from BeaverDeck\n  replicas: 2\n  enabled: true\n`,
  'Secret (Opaque)': `apiVersion: v1\nkind: Secret\nmetadata:\n  name: sample-secret\n  namespace: default\ntype: Opaque\nstringData:\n  username: admin\n  password: change-me\n`,
  'Deployment': `apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: sample-app\n  namespace: default\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app: sample-app\n  template:\n    metadata:\n      labels:\n        app: sample-app\n    spec:\n      containers:\n        - name: app\n          image: nginx:1.27\n          ports:\n            - containerPort: 80\n`,
  'Service (ClusterIP)': `apiVersion: v1\nkind: Service\nmetadata:\n  name: sample-service\n  namespace: default\nspec:\n  selector:\n    app: sample-app\n  ports:\n    - port: 80\n      targetPort: 80\n  type: ClusterIP\n`,
  'Ingress': `apiVersion: networking.k8s.io/v1\nkind: Ingress\nmetadata:\n  name: sample-ingress\n  namespace: default\nspec:\n  rules:\n    - host: app.example.com\n      http:\n        paths:\n          - path: /\n            pathType: Prefix\n            backend:\n              service:\n                name: sample-service\n                port:\n                  number: 80\n`,
  'Pod': `apiVersion: v1\nkind: Pod\nmetadata:\n  name: sample-pod\n  namespace: default\nspec:\n  containers:\n    - name: app\n      image: nginx:1.27\n      ports:\n        - containerPort: 80\n`,
  'PVC': `apiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: sample-pvc\n  namespace: default\nspec:\n  accessModes:\n    - ReadWriteOnce\n  resources:\n    requests:\n      storage: 1Gi\n  storageClassName: standard\n`,
  'CronJob': `apiVersion: batch/v1\nkind: CronJob\nmetadata:\n  name: sample-cron\n  namespace: default\nspec:\n  schedule: "*/5 * * * *"\n  jobTemplate:\n    spec:\n      template:\n        spec:\n          restartPolicy: OnFailure\n          containers:\n            - name: echo\n              image: busybox:1.36\n              command: ["/bin/sh", "-c", "date; echo hello"]\n`
};
