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
    section: 'Audit',
    items: [
      { id: 'insights-nodes', label: 'Node Insights' },
      { id: 'insights-workloads', label: 'Workload Insights' },
      { id: 'insights-networking', label: 'Network Insights' },
      { id: 'insights-storage', label: 'Storage Insights' },
      { id: 'cluster-health', label: 'Cluster Health' },
      { id: 'audit', label: 'Audit' }
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

export const AUTH_STORAGE_KEY = 'beaverdeck-auth';
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
  'insights-networking': { value: 'networking', label: 'Networking' },
  'insights-storage': { value: 'storage', label: 'Storage' }
};

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
  secrets: { key: 'name', dir: 'asc' },
  pvcs: { key: 'name', dir: 'asc' },
  pvs: { key: 'name', dir: 'asc' },
  storageclasses: { key: 'name', dir: 'asc' }
};

export const ROLE_RESOURCES = ['pods', 'workloads', 'nodes', 'services', 'clusterroles', 'rbacroles', 'serviceaccounts', 'ingresses', 'configmaps', 'crds', 'secrets', 'pvcs', 'pvs', 'storageclasses', 'events', 'insights', 'exec', 'apply', 'audit', 'users', 'roles'];
export const CLUSTER_SCOPED_RESOURCES = new Set(['nodes', 'crds', 'pvs', 'storageclasses']);
export const BOTTOM_DOCK_HIDDEN_NAVS = new Set(['insights', 'insights-nodes', 'insights-workloads', 'insights-networking', 'insights-storage', 'cluster-health', 'audit', 'user-management', 'apply']);
export const DOCK_TOP_RATIO_DEFAULT = 0.62;
export const DOCK_TOP_RATIO_MIN = 0.3;
export const DOCK_TOP_RATIO_MAX = 0.8;

export const NAV_RESOURCE = {
  pods: 'pods',
  workloads: 'workloads',
  nodes: 'nodes',
  events: 'events',
  insights: 'insights',
  'insights-nodes': 'insights',
  'insights-workloads': 'insights',
  'insights-networking': 'insights',
  'insights-storage': 'insights',
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
  apply: 'apply',
  audit: 'audit'
};

export const ROLE_RESOURCE_OPTIONS = {
  pods: [
    { value: 'none', label: 'No access', hint: 'Cannot view or manage pods.' },
    { value: 'view', label: 'View pods', hint: 'Can view pod lists, manifests and logs.' },
    { value: 'edit', label: 'Manage pods', hint: 'Can view and edit pods.' },
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
    { value: 'view', label: 'View CRDs', hint: 'Can view CustomResourceDefinitions and manifests.' },
    { value: 'edit', label: 'Manage CRDs', hint: 'Can edit CustomResourceDefinitions.' },
    { value: 'full', label: 'Full CRD access', hint: 'Can view, edit and delete CustomResourceDefinitions.' }
  ],
  secrets: [
    { value: 'none', label: 'No access', hint: 'Cannot view Secrets.' },
    { value: 'view', label: 'View secret list', hint: 'Can view Secrets list; secret content may still be restricted.' },
    { value: 'edit', label: 'Manage Secrets', hint: 'Can edit Secrets.' },
    { value: 'full', label: 'Full secret access', hint: 'Can view, edit and delete Secrets.' }
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
    { value: 'edit', label: 'Use exec', hint: 'Can open exec sessions in running pods.' }
  ],
  apply: [
    { value: 'none', label: 'No access', hint: 'Cannot use Apply YAML.' },
    { value: 'edit', label: 'Apply YAML', hint: 'Can dry-run and apply manifests.' }
  ],
  audit: [
    { value: 'none', label: 'No access', hint: 'Cannot view audit log.' },
    { value: 'view', label: 'View audit', hint: 'Can open the audit log.' }
  ],
  users: [
    { value: 'none', label: 'No access', hint: 'Cannot view users.' },
    { value: 'view', label: 'View users', hint: 'Can see users and assigned roles.' },
    { value: 'edit', label: 'Manage users', hint: 'Can create and update users.' },
    { value: 'full', label: 'Full user admin', hint: 'Can create, update and delete users.' }
  ],
  roles: [
    { value: 'none', label: 'No access', hint: 'Cannot view roles.' },
    { value: 'view', label: 'View roles', hint: 'Can see defined roles.' },
    { value: 'edit', label: 'Manage roles', hint: 'Can create and update roles.' },
    { value: 'full', label: 'Full role admin', hint: 'Can create, update and delete roles.' }
  ]
};

export const APPLY_TEMPLATES = {
  'ConfigMap': `apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sample-config\n  namespace: default\ndata:\n  APP_ENV: dev\n  LOG_LEVEL: info\n`,
  'Secret (Opaque)': `apiVersion: v1\nkind: Secret\nmetadata:\n  name: sample-secret\n  namespace: default\ntype: Opaque\nstringData:\n  username: admin\n  password: change-me\n`,
  'Deployment': `apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: sample-app\n  namespace: default\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app: sample-app\n  template:\n    metadata:\n      labels:\n        app: sample-app\n    spec:\n      containers:\n        - name: app\n          image: nginx:1.27\n          ports:\n            - containerPort: 80\n`,
  'Service (ClusterIP)': `apiVersion: v1\nkind: Service\nmetadata:\n  name: sample-service\n  namespace: default\nspec:\n  selector:\n    app: sample-app\n  ports:\n    - port: 80\n      targetPort: 80\n  type: ClusterIP\n`,
  'Ingress': `apiVersion: networking.k8s.io/v1\nkind: Ingress\nmetadata:\n  name: sample-ingress\n  namespace: default\nspec:\n  rules:\n    - host: app.example.com\n      http:\n        paths:\n          - path: /\n            pathType: Prefix\n            backend:\n              service:\n                name: sample-service\n                port:\n                  number: 80\n`,
  'Pod': `apiVersion: v1\nkind: Pod\nmetadata:\n  name: sample-pod\n  namespace: default\nspec:\n  containers:\n    - name: app\n      image: nginx:1.27\n      ports:\n        - containerPort: 80\n`,
  'PVC': `apiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: sample-pvc\n  namespace: default\nspec:\n  accessModes:\n    - ReadWriteOnce\n  resources:\n    requests:\n      storage: 1Gi\n  storageClassName: standard\n`,
  'CronJob': `apiVersion: batch/v1\nkind: CronJob\nmetadata:\n  name: sample-cron\n  namespace: default\nspec:\n  schedule: "*/5 * * * *"\n  jobTemplate:\n    spec:\n      template:\n        spec:\n          restartPolicy: OnFailure\n          containers:\n            - name: echo\n              image: busybox:1.36\n              command: ["/bin/sh", "-c", "date; echo hello"]\n`
};
