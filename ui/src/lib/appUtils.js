import {
  ROLE_RESOURCES,
  ROLE_RESOURCE_OPTIONS,
  THEME_STORAGE_PREFIX
} from './appConstants.js';

export function detectSystemTheme() {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return 'dark';
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

export function normalizeThemePreference(raw) {
  return ['auto', 'dark', 'light'].includes(raw) ? raw : 'auto';
}

export function getStoredThemePreference(username) {
  if (typeof window === 'undefined' || !username) return 'auto';
  try {
    return normalizeThemePreference(localStorage.getItem(`${THEME_STORAGE_PREFIX}${username}`));
  } catch {
    return 'auto';
  }
}

export function persistThemePreference(username, value) {
  if (typeof window === 'undefined' || !username) return;
  try {
    localStorage.setItem(`${THEME_STORAGE_PREFIX}${username}`, normalizeThemePreference(value));
  } catch {
    // Ignore storage failures.
  }
}

export function terminalThemeFor(mode) {
  if (mode === 'light') {
    return {
      background: '#f8fafc',
      foreground: '#0f172a',
      cursor: '#2563eb',
      selectionBackground: '#2563eb',
      selectionForeground: '#ffffff',
      selectionInactiveBackground: '#93c5fd'
    };
  }
  return {
    background: '#0a0f18',
    foreground: '#dbeafe',
    cursor: '#93c5fd',
    selectionBackground: '#2563eb',
    selectionForeground: '#ffffff',
    selectionInactiveBackground: '#1d4ed8'
  };
}

export function defaultRolePermissions() {
  const resources = {};
  ROLE_RESOURCES.forEach((resource) => {
    resources[resource] = { view: false, edit: false, delete: false };
  });
  return { namespaces: [], resources };
}

export function normalizeRolePermissions(raw) {
  const base = defaultRolePermissions();
  if (!raw || typeof raw !== 'object') return base;
  if (Array.isArray(raw.namespaces)) {
    base.namespaces = raw.namespaces.filter((item) => typeof item === 'string');
  }
  if (raw.resources && typeof raw.resources === 'object') {
    ROLE_RESOURCES.forEach((resource) => {
      const value = raw.resources[resource];
      if (typeof value === 'string') {
        base.resources[resource] = permissionFlags(value);
      } else if (value && typeof value === 'object') {
        const next = {
          view: Boolean(value.view),
          edit: Boolean(value.edit),
          delete: Boolean(value.delete)
        };
        if (next.delete) {
          next.edit = true;
          next.view = true;
        }
        if (next.edit) {
          next.view = true;
        }
        base.resources[resource] = next;
      }
    });
  }
  return base;
}

export function permissionLevel(permission) {
  if (permission?.delete) return 'full';
  if (permission?.edit) return 'edit';
  if (permission?.view) return 'view';
  return 'none';
}

export function permissionFlags(level) {
  switch (level) {
    case 'full':
      return { view: true, edit: true, delete: true };
    case 'edit':
      return { view: true, edit: true, delete: false };
    case 'view':
      return { view: true, edit: false, delete: false };
    default:
      return { view: false, edit: false, delete: false };
  }
}

export function roleOptionsFor(resource) {
  return ROLE_RESOURCE_OPTIONS[resource] || [
    { value: 'none', label: 'No access', hint: 'No access.' },
    { value: 'view', label: 'View', hint: 'View access.' },
    { value: 'edit', label: 'Edit', hint: 'Edit access.' },
    { value: 'full', label: 'Full', hint: 'Full access.' }
  ];
}

export function resolveRoleLevel(resource, preferred) {
  const options = roleOptionsFor(resource).map((item) => item.value);
  if (options.includes(preferred)) return preferred;
  if (preferred === 'full') {
    if (options.includes('edit')) return 'edit';
    if (options.includes('view')) return 'view';
    return 'none';
  }
  if (preferred === 'edit') {
    if (options.includes('view')) return 'view';
    return 'none';
  }
  if (preferred === 'view') {
    return options.includes('view') ? 'view' : 'none';
  }
  return 'none';
}

export function compareValues(a, b) {
  if (a == null && b == null) return 0;
  if (a == null) return -1;
  if (b == null) return 1;

  const normalizedA = String(a).toLowerCase();
  const normalizedB = String(b).toLowerCase();

  const numericA = Number(normalizedA);
  const numericB = Number(normalizedB);
  if (!Number.isNaN(numericA) && !Number.isNaN(numericB)) {
    return numericA - numericB;
  }

  return normalizedA.localeCompare(normalizedB);
}

export function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

export function formatMilliValue(value) {
  return value > 0 ? `${value}m` : '-';
}

export function desiredReplicasFromReady(ready) {
  const parts = String(ready || '').split('/');
  if (parts.length !== 2) return 1;
  const desired = Number(parts[1]);
  return Number.isFinite(desired) && desired >= 0 ? desired : 1;
}

export function summarizeApplyResult(data, dryRun) {
  const items = Array.isArray(data?.items) ? data.items : [];
  const modeLabel = dryRun ? 'Dry-run' : 'Applied';
  if (items.length === 0) {
    return dryRun ? 'Dry-run succeeded' : 'Apply succeeded';
  }
  if (items.length === 1) {
    const item = items[0] || {};
    const kind = item.kind || 'object';
    const namespace = item.namespace ? `${item.namespace}/` : '';
    const name = item.name || '';
    return `${modeLabel}: ${kind} ${namespace}${name}`.trim();
  }
  return `${modeLabel}: ${items.length} objects`;
}

export function formatByteValue(value) {
  return value > 0 ? formatBytesIEC(value) : '-';
}

export function formatGPURequestLabel(count) {
  if (!(count > 0)) return '-';
  return count === 1 ? '1 GPU' : `${count} GPUs`;
}

export function formatBytesIEC(value) {
  const size = Number(value || 0);
  if (!Number.isFinite(size) || size <= 0) return '-';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
  let current = size;
  let unitIndex = 0;
  while (current >= 1024 && unitIndex < units.length - 1) {
    current /= 1024;
    unitIndex += 1;
  }
  return `${current.toFixed(current >= 10 || unitIndex === 0 ? 0 : 1)}${units[unitIndex]}`;
}

export function kindToResource(kind) {
  const normalized = String(kind || '').toLowerCase();
  if (normalized.startsWith('customresource:')) return 'crds';
  if (['pod', 'pods'].includes(normalized)) return 'pods';
  if (['deployment', 'deployments', 'daemonset', 'daemonsets', 'statefulset', 'statefulsets', 'job', 'jobs', 'cronjob', 'cronjobs', 'replicaset', 'replicasets', 'replicationcontroller', 'replicationcontrollers'].includes(normalized)) return 'workloads';
  if (['node', 'nodes'].includes(normalized)) return 'nodes';
  if (['service', 'services'].includes(normalized)) return 'services';
  if (['clusterrole', 'clusterroles', 'clusterrolebinding', 'clusterrolebindings'].includes(normalized)) return 'clusterroles';
  if (['role', 'roles', 'rolebinding', 'rolebindings'].includes(normalized)) return 'rbacroles';
  if (['serviceaccount', 'serviceaccounts'].includes(normalized)) return 'serviceaccounts';
  if (['ingress', 'ingresses'].includes(normalized)) return 'ingresses';
  if (['configmap', 'configmaps'].includes(normalized)) return 'configmaps';
  if (['crd', 'customresourcedefinition', 'customresourcedefinitions'].includes(normalized)) return 'crds';
  if (['secret', 'secrets'].includes(normalized)) return 'secrets';
  if (['pvc', 'persistentvolumeclaim', 'persistentvolumeclaims'].includes(normalized)) return 'pvcs';
  if (['pv', 'persistentvolume', 'persistentvolumes'].includes(normalized)) return 'pvs';
  if (['storageclass', 'storageclasses'].includes(normalized)) return 'storageclasses';
  return '';
}

export function displayKind(kind) {
  const normalized = String(kind || '').trim().toLowerCase();
  if (normalized.startsWith('customresource:')) {
    return String(kind || '').trim().slice('customresource:'.length) || 'CustomResource';
  }
  const labels = {
    pod: 'Pod',
    deployment: 'Deployment',
    statefulset: 'StatefulSet',
    daemonset: 'DaemonSet',
    job: 'Job',
    cronjob: 'CronJob',
    node: 'Node',
    service: 'Service',
    ingress: 'Ingress',
    configmap: 'ConfigMap',
    crd: 'CRD',
    customresourcedefinition: 'CRD',
    secret: 'Secret',
    pvc: 'PVC',
    persistentvolumeclaim: 'PVC',
    pv: 'PV',
    persistentvolume: 'PV',
    storageclass: 'StorageClass',
    serviceaccount: 'ServiceAccount',
    role: 'Role',
    rolebinding: 'RoleBinding',
    clusterrole: 'ClusterRole',
    clusterrolebinding: 'ClusterRoleBinding'
  };
  return labels[normalized] || String(kind || '');
}
