import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import {
  CreateUserModal,
  DrainModal,
  GoogleConfigModal,
  GoogleMappingsModal,
  OIDCConfigModal,
  OIDCMappingsModal,
  PasswordPromptModal,
  ProfileModal,
  RoleModal,
  ScaleModal
} from './components/AppModals.jsx';
import ApplyYamlPage from './components/ApplyYamlPage.jsx';
import BottomDock from './components/BottomDock.jsx';
import { BootstrapSetupScreen, LoginScreen, SidebarNav, WorkspaceHeader } from './components/AppChrome.jsx';
import ClusterHealthPage from './components/ClusterHealthPage.jsx';
import InsightsPage from './components/InsightsPage.jsx';
import NodesPage from './components/NodesPage.jsx';
import PodsPage from './components/PodsPage.jsx';
import { ClusterRolesPage, NamespacedRolesPage, ServiceAccountsPage } from './components/RbacPages.jsx';
import {
  CRDsPage,
  ConfigMapsPage,
  EventsPage,
  IngressesPage,
  PVCsPage,
  PVsPage,
  SecretsPage,
  ServicesPage,
  StorageClassesPage
} from './components/ResourcePages.jsx';
import UserManagementPage from './components/UserManagementPage.jsx';
import WarningPopover from './components/WarningPopover.jsx';
import WorkloadsPage from './components/WorkloadsPage.jsx';
import useAuthSession from './hooks/useAuthSession.js';
import useThemeMode from './hooks/useThemeMode.js';
import {
  APPLY_TEMPLATES,
  BOTTOM_DOCK_HIDDEN_NAVS,
  CLUSTER_SCOPED_RESOURCES,
  DOCK_TOP_RATIO_DEFAULT,
  DOCK_TOP_RATIO_MAX,
  DOCK_TOP_RATIO_MIN,
  INSIGHT_NAV_CATEGORIES,
  MENU,
  NAV_RESOURCE,
  ROLE_RESOURCES,
  SORT_DEFAULTS,
  THEME_OPTIONS
} from './lib/appConstants.js';
import {
  clamp,
  compareValues,
  defaultRolePermissions,
  desiredReplicasFromReady,
  displayKind,
  formatByteValue,
  formatBytesIEC,
  formatGPURequestLabel,
  formatMilliValue,
  kindToResource,
  normalizeRolePermissions,
  permissionFlags,
  permissionLevel,
  persistThemePreference,
  resolveRoleLevel,
  roleOptionsFor,
  summarizeApplyResult,
  terminalThemeFor
} from './lib/appUtils.js';
import { withBasePath } from './lib/paths.js';

const ACTIVE_NAV_STORAGE_KEY = 'beaverdeck.activeNav';
const DEFAULT_EXPANDED_NAV_SECTIONS = new Set();

function groupCRDsForNavigation(crds) {
  const groups = new Map();
  crds.forEach((crd) => {
    const group = crd.group || 'Other';
    if (!groups.has(group)) groups.set(group, []);
    groups.get(group).push(crd);
  });
  return [...groups.entries()]
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([group, groupCRDs]) => ({
      id: group,
      label: group,
      children: groupCRDs
        .sort((left, right) => (left.kind || left.name).localeCompare(right.kind || right.name))
        .map((crd) => ({ id: crd.name, label: crd.kind || crd.name }))
    }));
}

const DEFAULT_OIDC_CONFIG = {
  provider_name: 'OpenID Connect',
  issuer_url: '',
  client_id: '',
  client_secret: '',
  scopes: 'openid email profile groups',
  hosted_domain: '',
  email_claim: 'email',
  groups_claim: 'groups'
};

const DEFAULT_ENTRA_CONFIG = {
  provider_name: 'Azure Entra ID',
  issuer_url: '',
  client_id: '',
  client_secret: '',
  scopes: 'openid email profile User.Read GroupMember.Read.All',
  hosted_domain: '',
  email_claim: 'email',
  groups_claim: 'groups'
};

function isEntraOIDCConfig(config) {
  return /entra|azure|microsoftonline\.com|sts\.windows\.net/i.test(`${config?.provider_name || ''} ${config?.issuer_url || ''}`);
}

function normalizeOIDCConfig(config, fallback = DEFAULT_OIDC_CONFIG) {
  return {
    provider_name: config?.provider_name || fallback.provider_name,
    issuer_url: config?.issuer_url || '',
    client_id: config?.client_id || '',
    client_secret: config?.client_secret || '',
    scopes: config?.scopes || fallback.scopes,
    hosted_domain: config?.hosted_domain || '',
    email_claim: config?.email_claim || fallback.email_claim,
    groups_claim: config?.groups_claim || fallback.groups_claim
  };
}

function loadPersistedActiveNav() {
  try {
    const value = window.localStorage.getItem(ACTIVE_NAV_STORAGE_KEY);
    return value === 'insights' ? 'insights-nodes' : (value || 'pods');
  } catch {
    return 'pods';
  }
}

function navSectionForId(menu, navId) {
  return menu.find((group) => group.items.some((item) => item.id === navId))?.section || '';
}

function bottomTabResourceKey(namespace, kind, name) {
  const normalizedKind = String(kind || '').trim().toLowerCase();
  const resource = kindToResource(normalizedKind);
  const isClusterScoped = resource === 'clusterroles' || CLUSTER_SCOPED_RESOURCES.has(resource);
  const normalizedNamespace = isClusterScoped ? '' : String(namespace || '').trim().toLowerCase();
  return `${normalizedNamespace}:${normalizedKind}:${String(name || '').trim()}`;
}

function normalizeContainerNames(containers) {
  if (!Array.isArray(containers)) {
    return [];
  }
  return Array.from(new Set(containers.map((item) => String(item || '').trim()).filter(Boolean))).sort((a, b) => a.localeCompare(b));
}

function podRefKey(namespace, name) {
  return `${String(namespace || '').trim()}/${String(name || '').trim()}`;
}

export default function App() {
  const [status, setStatus] = useState('');
  const [bottomNotice, setBottomNotice] = useState(null);
  const [showProfile, setShowProfile] = useState(false);

  const [activeNav, setActiveNav] = useState(loadPersistedActiveNav);
  const [expandedNavSections, setExpandedNavSections] = useState(() => new Set(DEFAULT_EXPANDED_NAV_SECTIONS));
  const [loadedNavs, setLoadedNavs] = useState({});
  const [initialNavLoading, setInitialNavLoading] = useState('');
  const [sortByNav, setSortByNav] = useState(SORT_DEFAULTS);

  const [namespaces, setNamespaces] = useState([]);
  const [selectedNamespaces, setSelectedNamespaces] = useState([]);

  const [workloads, setWorkloads] = useState([]);
  const [pods, setPods] = useState([]);
  const [healthPods, setHealthPods] = useState([]);
  const [nodes, setNodes] = useState([]);
  const [events, setEvents] = useState([]);
  const [ingresses, setIngresses] = useState([]);
  const [services, setServices] = useState([]);
  const [clusterRoles, setClusterRoles] = useState([]);
  const [clusterRoleBindings, setClusterRoleBindings] = useState([]);
  const [rbacRoles, setRbacRoles] = useState([]);
  const [roleBindings, setRoleBindings] = useState([]);
  const [serviceAccounts, setServiceAccounts] = useState([]);
  const [configMaps, setConfigMaps] = useState([]);
  const [crds, setCRDs] = useState([]);
  const [selectedCRDName, setSelectedCRDName] = useState('');
  const [selectedCRDGroup, setSelectedCRDGroup] = useState('');
  const [customResources, setCustomResources] = useState([]);
  const [customResourceDefinition, setCustomResourceDefinition] = useState(null);
  const [crdNavExpanded, setCRDNavExpanded] = useState(false);
  const [expandedCRDGroups, setExpandedCRDGroups] = useState(() => new Set());
  const [secrets, setSecrets] = useState([]);
  const [pvcs, setPVCs] = useState([]);
  const [pvs, setPVs] = useState([]);
  const [storageClasses, setStorageClasses] = useState([]);
  const [insights, setInsights] = useState([]);
  const [showSuppressedInsights, setShowSuppressedInsights] = useState(false);
  const [showAllInsightChecks, setShowAllInsightChecks] = useState(false);
  const [selectedInsightTypes, setSelectedInsightTypes] = useState([]);
  const [warningPopover, setWarningPopover] = useState(null);
  const [warningCache, setWarningCache] = useState({});
  const [managedUsers, setManagedUsers] = useState([]);
  const [managedRoles, setManagedRoles] = useState([]);
  const [googleConfig, setGoogleConfig] = useState({
    client_id: '',
    client_secret: '',
    hosted_domain: '',
    service_account_json: '',
    delegated_admin_email: ''
  });
  const [googleMappings, setGoogleMappings] = useState([]);
  const [oidcConfig, setOIDCConfig] = useState({
    provider_name: 'OpenID Connect',
    issuer_url: '',
    client_id: '',
    client_secret: '',
    scopes: 'openid email profile groups',
    hosted_domain: '',
    email_claim: 'email',
    groups_claim: 'groups'
  });
  const [oidcConfigDraft, setOIDCConfigDraft] = useState(DEFAULT_OIDC_CONFIG);
  const [oidcMappings, setOIDCMappings] = useState([]);
  const [newGoogleGroupEmail, setNewGoogleGroupEmail] = useState('');
  const [newGoogleRole, setNewGoogleRole] = useState('viewer');
  const [editingGoogleGroupEmail, setEditingGoogleGroupEmail] = useState('');
  const [newOIDCGroupName, setNewOIDCGroupName] = useState('');
  const [newOIDCRole, setNewOIDCRole] = useState('viewer');
  const [editingOIDCGroupName, setEditingOIDCGroupName] = useState('');
  const [newUsername, setNewUsername] = useState('');
  const [newUserPassword, setNewUserPassword] = useState('');
  const [newUserRole, setNewUserRole] = useState('viewer');
  const [showCreateUserModal, setShowCreateUserModal] = useState(false);
  const [showRoleModal, setShowRoleModal] = useState(false);
  const [showGoogleConfigModal, setShowGoogleConfigModal] = useState(false);
  const [showGoogleMappingsModal, setShowGoogleMappingsModal] = useState(false);
  const [showOIDCConfigModal, setShowOIDCConfigModal] = useState(false);
  const [oidcConfigModalMode, setOIDCConfigModalMode] = useState('oidc');
  const [showOIDCMappingsModal, setShowOIDCMappingsModal] = useState(false);
  const [editingRoleName, setEditingRoleName] = useState('');
  const [roleFormName, setRoleFormName] = useState('');
  const [roleFormMode, setRoleFormMode] = useState('viewer');
  const [roleFormNamespaces, setRoleFormNamespaces] = useState([]);
  const [roleFormPermissions, setRoleFormPermissions] = useState({});
  const [showDrainModal, setShowDrainModal] = useState(false);
  const [drainTargetNode, setDrainTargetNode] = useState('');
  const [drainForce, setDrainForce] = useState(false);
  const [showScaleModal, setShowScaleModal] = useState(false);

  const [podSearch, setPodSearch] = useState('');
  const [workloadSearch, setWorkloadSearch] = useState('');
  const [serviceSearch, setServiceSearch] = useState('');
  const [ingressSearch, setIngressSearch] = useState('');
  const [serviceAccountSearch, setServiceAccountSearch] = useState('');
  const [configMapSearch, setConfigMapSearch] = useState('');
  const [crdSearch, setCRDSearch] = useState('');
  const [customResourceSearch, setCustomResourceSearch] = useState('');
  const [secretSearch, setSecretSearch] = useState('');
  const [pvcSearch, setPVCSearch] = useState('');
  const [pvSearch, setPVSearch] = useState('');
  const [podsAutoRefreshEnabled, setPodsAutoRefreshEnabled] = useState(true);
  const [podsAutoRefreshSeconds, setPodsAutoRefreshSeconds] = useState(15);
  const [podStatusFilter, setPodStatusFilter] = useState('');
  const [selectedPod, setSelectedPod] = useState(null);
  const [selectedPodRefs, setSelectedPodRefs] = useState([]);

  const [deploymentName, setDeploymentName] = useState('');
  const [deploymentNamespace, setDeploymentNamespace] = useState('');
  const [scaleTargetKind, setScaleTargetKind] = useState('Deployment');
  const [replicas, setReplicas] = useState(1);
  const [nodeLabelFilter, setNodeLabelFilter] = useState('');

  const [yamlText, setYamlText] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState('');
  const [bootstrapAdminUsername, setBootstrapAdminUsername] = useState('admin');
  const [bootstrapAdminPassword, setBootstrapAdminPassword] = useState('');
  const [bootstrapAdminPasswordConfirm, setBootstrapAdminPasswordConfirm] = useState('');
  const [bootstrapError, setBootstrapError] = useState('');
  const [showPasswordPromptModal, setShowPasswordPromptModal] = useState(false);
  const [passwordPromptUsername, setPasswordPromptUsername] = useState('');
  const [passwordPromptValue, setPasswordPromptValue] = useState('');
  const [passwordPromptConfirm, setPasswordPromptConfirm] = useState('');

  const [bottomTabs, setBottomTabs] = useState([]);
  const [activeBottomTabId, setActiveBottomTabId] = useState('');
  const [dockSplitRatio, setDockSplitRatio] = useState(DOCK_TOP_RATIO_DEFAULT);
  const bottomTabsRef = useRef([]);
  const execSocketsRef = useRef({});
  const execTerminalRef = useRef(null);
  const execTerminalHostRef = useRef(null);
  const workspaceMainRef = useRef(null);
  const dockResizeStateRef = useRef(null);
  const podsAutoRefreshBusyRef = useRef(false);
  const warningHideTimerRef = useRef(null);
  const logFollowRef = useRef(null);
  const logsOutputRef = useRef(null);
  const logsEndRef = useRef(null);
  const forceLogScrollRef = useRef(false);
  const pendingLogPrependRef = useRef(null);
  const bottomNoticeTimerRef = useRef(null);
  const configImportInputRef = useRef(null);

  const {
    themePreference,
    setThemePreference,
    resolvedTheme
  } = useThemeMode();

  const {
    api,
    usernameInput,
    setUsernameInput,
    passwordInput,
    setPasswordInput,
    token,
    username,
    isLoggedIn,
    authBootstrapping,
    authProviders,
    authError,
    bootstrapState,
    currentUser,
    login,
    completeBootstrap,
    logout,
    reloadAuthProviders,
    startGoogleLogin,
    startOIDCLogin
  } = useAuthSession({
    selectedNamespaces,
    setNamespaces,
    setSelectedNamespaces,
    setThemePreference,
    beforeLogout: async () => {
      Object.keys(execSocketsRef.current).forEach((id) => {
        try {
          execSocketsRef.current[id]?.close();
        } catch {
          // ignore close errors
        }
      });
      execSocketsRef.current = {};
      if (execTerminalRef.current) {
        execTerminalRef.current.dispose();
        execTerminalRef.current = null;
      }
    },
    afterLogout: () => {
      setStatus('');
      setShowProfile(false);
      setActiveNav('pods');
      setExpandedNavSections(new Set(DEFAULT_EXPANDED_NAV_SECTIONS));
      setLoadedNavs({});
      setInitialNavLoading('');
      setWorkloads([]);
      setPods([]);
      setHealthPods([]);
      setNodes([]);
      setEvents([]);
      setIngresses([]);
      setServices([]);
      setClusterRoles([]);
      setClusterRoleBindings([]);
      setRbacRoles([]);
      setRoleBindings([]);
      setServiceAccounts([]);
      setConfigMaps([]);
      setCRDs([]);
      setSelectedCRDName('');
      setSelectedCRDGroup('');
      setCustomResources([]);
      setCustomResourceDefinition(null);
      setExpandedCRDGroups(new Set());
      setSecrets([]);
      setPVCs([]);
      setPVs([]);
      setStorageClasses([]);
      setInsights([]);
      setShowSuppressedInsights(false);
      setManagedUsers([]);
      setManagedRoles([]);
      setGoogleConfig({
        client_id: '',
        client_secret: '',
        hosted_domain: '',
        service_account_json: '',
        delegated_admin_email: ''
      });
      setGoogleMappings([]);
      setOIDCConfig({
        ...DEFAULT_OIDC_CONFIG
      });
      setOIDCConfigDraft({ ...DEFAULT_OIDC_CONFIG });
      setOIDCMappings([]);
      setShowGoogleConfigModal(false);
      setShowGoogleMappingsModal(false);
      setShowOIDCConfigModal(false);
      setShowOIDCMappingsModal(false);
      setShowCreateUserModal(false);
      setBootstrapAdminUsername('admin');
      setBootstrapAdminPassword('');
      setBootstrapAdminPasswordConfirm('');
      setBootstrapError('');
      setShowPasswordPromptModal(false);
      setPasswordPromptUsername('');
      setPasswordPromptValue('');
      setPasswordPromptConfirm('');
      resetGoogleMappingForm();
      resetOIDCMappingForm();
      setBottomTabs([]);
      setActiveBottomTabId('');
      setSelectedPodRefs([]);
    }
  });
  const isAdmin = currentUser.roleMode === 'admin';
  const userPermissions = useMemo(() => normalizeRolePermissions(currentUser.permissions), [currentUser.permissions]);
  const primaryNamespace = useMemo(() => selectedNamespaces[0] || '', [selectedNamespaces]);
  const namespaceQuery = useMemo(() => selectedNamespaces.join(','), [selectedNamespaces]);
  const permissionInfo = (resource, action, namespace) => {
    if (isAdmin) return { allowed: true, reason: '' };
    const perm = userPermissions.resources?.[resource];
    if (!perm || !perm[action]) return { allowed: false, reason: `Role has no ${action} permission for ${resource}` };
    if (!namespace || CLUSTER_SCOPED_RESOURCES.has(resource)) return { allowed: true, reason: '' };
    const allowedNs = userPermissions.namespaces || [];
    if (allowedNs.length === 0) return { allowed: true, reason: '' };
    if (!allowedNs.includes(namespace)) {
      return { allowed: false, reason: `Namespace ${namespace} is not allowed by role` };
    }
    return { allowed: true, reason: '' };
  };
  const hasPermission = (resource, action, namespace) => permissionInfo(resource, action, namespace).allowed;
  const allAllowed = (...checks) => {
    for (const c of checks) {
      if (!c?.allowed) return c;
    }
    return { allowed: true, reason: '' };
  };
  const makeAction = (label, check, onClick) => ({ label, enabled: check.allowed, reason: check.reason, onClick });
  const canAccessNav = (id) => {
    if (id === 'user-management' || id === 'cluster-health') return isAdmin;
    const resource = NAV_RESOURCE[id];
    if (!resource) return true;
    const action = id === 'apply' ? 'edit' : 'view';
    return hasPermission(resource, action);
  };
  const visibleMenu = useMemo(
    () => MENU
      .filter((group) => group.section !== 'Admin' || isAdmin)
      .map((group) => ({
        ...group,
        items: group.items
          .filter((item) => canAccessNav(item.id))
          .map((item) => item.id === 'crds'
            ? {
                ...item,
                children: groupCRDsForNavigation(crds)
              }
            : item)
      }))
      .filter((group) => group.items.length > 0),
    [isAdmin, userPermissions, crds]
  );
  const visibleNavItems = useMemo(() => visibleMenu.flatMap((group) => group.items), [visibleMenu]);
  const expandNavSectionForNav = useCallback((navId) => {
    const section = navSectionForId(visibleMenu, navId);
    if (!section) return;
    setExpandedNavSections((prev) => {
      if (prev.has(section)) return prev;
      const next = new Set(prev);
      next.add(section);
      return next;
    });
  }, [visibleMenu]);
  const activateNav = useCallback((nextNav) => {
    if (!nextNav) return;
    expandNavSectionForNav(nextNav);
    if (nextNav !== activeNav) {
      setActiveNav(nextNav);
    }
  }, [activeNav, expandNavSectionForNav]);
  const toggleNavSection = useCallback((section) => {
    setExpandedNavSections((prev) => {
      const next = new Set(prev);
      if (next.has(section)) {
        next.delete(section);
      } else {
        next.add(section);
      }
      return next;
    });
  }, []);

  const activeBottomTab = useMemo(() => bottomTabs.find((t) => t.id === activeBottomTabId), [bottomTabs, activeBottomTabId]);
  const activeInsightCategory = INSIGHT_NAV_CATEGORIES[activeNav]?.value || 'nodes';
  const activeInsightCategoryLabel = INSIGHT_NAV_CATEGORIES[activeNav]?.label || 'Nodes';
  const isInsightsView = Boolean(INSIGHT_NAV_CATEGORIES[activeNav]);
  const showInitialNavLoader = initialNavLoading === activeNav && !loadedNavs[activeNav];
  const hasBottomDock = bottomTabs.length > 0;
  const showBottomDock = hasBottomDock && !BOTTOM_DOCK_HIDDEN_NAVS.has(activeNav);
  const isPodsView = activeNav === 'pods';
  const googleAuthConfigured = Boolean(
    googleConfig.client_id.trim() &&
    googleConfig.client_secret.trim() &&
    googleConfig.service_account_json.trim() &&
    googleConfig.delegated_admin_email.trim()
  );
  const oidcAuthConfigured = Boolean(
    oidcConfig.issuer_url.trim() &&
    oidcConfig.client_id.trim() &&
    oidcConfig.client_secret.trim()
  );

  const podNameRegex = podSearch.trim();
  const podNameRegexError = useMemo(() => {
    if (!podNameRegex) return '';
    try {
      // Validate user regex once and reuse it in filtering.
      // eslint-disable-next-line no-new
      new RegExp(podNameRegex, 'i');
      return '';
    } catch (err) {
      return err.message || 'Invalid regex';
    }
  }, [podNameRegex]);
  const availablePodStatuses = useMemo(
    () => Array.from(new Set(pods.map((pod) => String(pod.phase || '').trim()).filter(Boolean))).sort((a, b) => a.localeCompare(b)),
    [pods]
  );
  const filteredPods = useMemo(() => {
    const nameRegex = podNameRegex && !podNameRegexError ? new RegExp(podNameRegex, 'i') : null;
    return pods.filter((pod) => {
      if (podStatusFilter && String(pod.phase || '') !== podStatusFilter) {
        return false;
      }
      if (podNameRegexError) {
        return false;
      }
      if (nameRegex && !nameRegex.test(String(pod.name || ''))) {
        return false;
      }
      return true;
    });
  }, [pods, podStatusFilter, podNameRegex, podNameRegexError]);
  const selectedPodRefSet = useMemo(() => new Set(selectedPodRefs), [selectedPodRefs]);
  const selectedPods = useMemo(
    () => filteredPods.filter((pod) => selectedPodRefSet.has(podRefKey(pod.namespace, pod.name))),
    [filteredPods, selectedPodRefSet]
  );

  function filterRowsByQuery(rows, query, fields) {
    const q = String(query || '').trim().toLowerCase();
    if (!q) return rows;
    return rows.filter((row) => fields.some((field) => String(row?.[field] ?? '').toLowerCase().includes(q)));
  }

  useEffect(() => {
    if (!isLoggedIn || !api) return;
    const targetNav = activeNav;
    const firstLoad = !loadedNavs[targetNav];
    if (firstLoad) {
      setInitialNavLoading(targetNav);
    }
    safe(refreshAll).finally(() => {
      setLoadedNavs((prev) => ({ ...prev, [targetNav]: true }));
      setInitialNavLoading((prev) => (prev === targetNav ? '' : prev));
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeNav, namespaceQuery, isLoggedIn, token]);

  useEffect(() => {
    if (!isLoggedIn || !api || activeNav !== 'pods' || !podsAutoRefreshEnabled || selectedNamespaces.length === 0) {
      return undefined;
    }
    const intervalMs = Math.max(1, Number(podsAutoRefreshSeconds) || 15) * 1000;
    const intervalId = window.setInterval(() => {
      if (podsAutoRefreshBusyRef.current) {
        return;
      }
      podsAutoRefreshBusyRef.current = true;
      refreshAll()
        .catch((err) => {
          setStatus(err.message || String(err));
        })
        .finally(() => {
          podsAutoRefreshBusyRef.current = false;
        });
    }, intervalMs);
    return () => window.clearInterval(intervalId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeNav, namespaceQuery, isLoggedIn, token, podsAutoRefreshEnabled, podsAutoRefreshSeconds]);

  useEffect(() => {
    if (!visibleNavItems.find((x) => x.id === activeNav)) {
      activateNav(visibleNavItems[0]?.id || 'pods');
    }
  }, [visibleNavItems, activeNav, activateNav]);

  useEffect(() => () => {
    Object.keys(execSocketsRef.current).forEach((id) => {
      try {
        execSocketsRef.current[id]?.close();
      } catch {
        // ignore close errors
      }
    });
    execSocketsRef.current = {};
    if (execTerminalRef.current) {
      execTerminalRef.current.dispose();
      execTerminalRef.current = null;
    }
    if (logFollowRef.current) {
      clearInterval(logFollowRef.current);
      logFollowRef.current = null;
    }
    if (warningHideTimerRef.current) {
      clearTimeout(warningHideTimerRef.current);
      warningHideTimerRef.current = null;
    }
    if (bottomNoticeTimerRef.current) {
      clearTimeout(bottomNoticeTimerRef.current);
      bottomNoticeTimerRef.current = null;
    }
  }, []);

  useEffect(() => {
    bottomTabsRef.current = bottomTabs;
  }, [bottomTabs]);

  useEffect(() => {
    const availableRefs = new Set(pods.map((pod) => podRefKey(pod.namespace, pod.name)));
    setSelectedPodRefs((prev) => {
      const next = prev.filter((ref) => availableRefs.has(ref));
      return next.length === prev.length ? prev : next;
    });
  }, [pods]);

  useEffect(() => {
    const visibleRefs = new Set(filteredPods.map((pod) => podRefKey(pod.namespace, pod.name)));
    setSelectedPodRefs((prev) => {
      const next = prev.filter((ref) => visibleRefs.has(ref));
      return next.length === prev.length ? prev : next;
    });
  }, [filteredPods]);

  useEffect(() => {
    setSelectedPodRefs([]);
  }, [namespaceQuery]);

  useEffect(() => {
    if (selectedPodRefs.length > 0 && selectedPod) {
      setSelectedPod(null);
    }
  }, [selectedPodRefs, selectedPod]);

  useEffect(() => {
    if (!isLoggedIn || !username) return;
    persistThemePreference(username, themePreference);
  }, [isLoggedIn, username, themePreference]);

  useEffect(() => {
    try {
      window.localStorage.setItem(ACTIVE_NAV_STORAGE_KEY, activeNav);
    } catch {
      // ignore storage errors
    }
  }, [activeNav]);

  useEffect(() => {
    if (!activeBottomTab || activeBottomTab.type !== 'exec' || !execTerminalHostRef.current) {
      if (execTerminalRef.current) {
        execTerminalRef.current.dispose();
        execTerminalRef.current = null;
      }
      return;
    }

    if (execTerminalRef.current?.tabId === activeBottomTab.id) {
      execTerminalRef.current.term.options.disableStdin = !activeBottomTab.connected;
      execTerminalRef.current.term.options.cursorBlink = Boolean(activeBottomTab.connected);
      execTerminalRef.current.fitAddon.fit();
      if (activeBottomTab.connected) {
        execTerminalRef.current.term.focus();
      }
      return;
    }

    if (execTerminalRef.current) {
      execTerminalRef.current.dispose();
      execTerminalRef.current = null;
    }

    const term = new Terminal({
      cursorBlink: Boolean(activeBottomTab.connected),
      convertEol: false,
      disableStdin: !activeBottomTab.connected,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
      fontSize: 13,
      theme: terminalThemeFor(resolvedTheme)
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    execTerminalHostRef.current.innerHTML = '';
    term.open(execTerminalHostRef.current);
    const fitAndResize = () => {
      fitAddon.fit();
      sendExecResize(activeBottomTab.id, term.cols, term.rows);
    };
    fitAndResize();
    term.write(activeBottomTab.content || '');
    term.focus();

    const dataDisposable = term.onData((data) => {
      const execTab = bottomTabsRef.current.find((tab) => tab.id === activeBottomTab.id);
      if (!execTab?.connected) {
        return;
      }
      sendExecData(activeBottomTab.id, data);
    });
    const termResizeDisposable = term.onResize(({ cols, rows }) => {
      sendExecResize(activeBottomTab.id, cols, rows);
    });
    const resizeObserver = typeof ResizeObserver !== 'undefined'
      ? new ResizeObserver(() => fitAndResize())
      : null;
    if (resizeObserver) {
      resizeObserver.observe(execTerminalHostRef.current);
    }
    const resizeHandler = () => fitAndResize();
    window.addEventListener('resize', resizeHandler);

    execTerminalRef.current = {
      tabId: activeBottomTab.id,
      term,
      fitAddon,
      dispose: () => {
        dataDisposable.dispose();
        termResizeDisposable.dispose();
        resizeObserver?.disconnect();
        window.removeEventListener('resize', resizeHandler);
        term.dispose();
      }
    };
  }, [activeBottomTab, resolvedTheme]);

  useEffect(() => {
    if (!execTerminalRef.current?.term) {
      return;
    }
    execTerminalRef.current.term.options.theme = terminalThemeFor(resolvedTheme);
  }, [resolvedTheme]);

  useEffect(() => {
    if (!execTerminalRef.current?.term || activeBottomTab?.type !== 'exec') {
      return;
    }
    execTerminalRef.current.term.options.disableStdin = !activeBottomTab.connected;
    execTerminalRef.current.term.options.cursorBlink = Boolean(activeBottomTab.connected);
  }, [activeBottomTab?.id, activeBottomTab?.type, activeBottomTab?.connected]);

  useEffect(() => {
    if (!showBottomDock || activeBottomTab?.type !== 'exec' || !execTerminalRef.current) {
      return undefined;
    }
    const rafId = window.requestAnimationFrame(() => {
      const terminal = execTerminalRef.current;
      terminal?.fitAddon?.fit();
      if (terminal?.term && terminal?.tabId) {
        sendExecResize(terminal.tabId, terminal.term.cols, terminal.term.rows);
      }
    });
    return () => window.cancelAnimationFrame(rafId);
  }, [activeBottomTab?.id, activeBottomTab?.type, dockSplitRatio, showBottomDock]);

  useEffect(() => {
    if (!showBottomDock) {
      dockResizeStateRef.current = null;
      document.body.classList.remove('is-resizing-dock');
      return undefined;
    }

    const stopResize = () => {
      dockResizeStateRef.current = null;
      document.body.classList.remove('is-resizing-dock');
    };

    const handleMouseMove = (event) => {
      if (!dockResizeStateRef.current || !workspaceMainRef.current) {
        return;
      }
      const rect = workspaceMainRef.current.getBoundingClientRect();
      if (rect.height <= 0) {
        return;
      }
      const nextRatio = clamp((event.clientY - rect.top) / rect.height, DOCK_TOP_RATIO_MIN, DOCK_TOP_RATIO_MAX);
      setDockSplitRatio(nextRatio);
    };

    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', stopResize);
    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', stopResize);
      document.body.classList.remove('is-resizing-dock');
    };
  }, [showBottomDock]);

  useEffect(() => {
    if (logFollowRef.current) {
      clearInterval(logFollowRef.current);
      logFollowRef.current = null;
    }
    if (!activeBottomTab || activeBottomTab.type !== 'logs' || !activeBottomTab.follow) {
      return undefined;
    }
    const poll = () => {
      void refreshLogTab(activeBottomTab.id, true);
    };
    poll();
    logFollowRef.current = window.setInterval(poll, 2500);
    return () => {
      if (logFollowRef.current) {
        clearInterval(logFollowRef.current);
        logFollowRef.current = null;
      }
    };
  }, [activeBottomTab?.id, activeBottomTab?.type, activeBottomTab?.follow]);

  useEffect(() => {
    const shouldScroll = Boolean(activeBottomTab?.follow) || (forceLogScrollRef.current && !activeBottomTab?.loading);
    if (!activeBottomTab || activeBottomTab.type !== 'logs' || !logsOutputRef.current || !shouldScroll) {
      return;
    }
    forceLogScrollRef.current = false;
    const rafId = window.requestAnimationFrame(() => {
      scrollLogsToBottom();
    });
    return () => window.cancelAnimationFrame(rafId);
  }, [activeBottomTab?.id, activeBottomTab?.type, activeBottomTab?.follow, activeBottomTab?.content, activeBottomTab?.loading]);

  useEffect(() => {
    const pending = pendingLogPrependRef.current;
    if (!pending || !activeBottomTab || activeBottomTab.type !== 'logs' || activeBottomTab.id !== pending.tabId || activeBottomTab.loadingOlder) {
      return undefined;
    }
    const wrap = logsOutputRef.current;
    if (!wrap) {
      pendingLogPrependRef.current = null;
      return undefined;
    }
    const rafId = window.requestAnimationFrame(() => {
      wrap.scrollTop = Math.max(0, wrap.scrollHeight - pending.prevHeight + pending.prevTop);
      pendingLogPrependRef.current = null;
    });
    return () => window.cancelAnimationFrame(rafId);
  }, [activeBottomTab?.id, activeBottomTab?.type, activeBottomTab?.content, activeBottomTab?.loadingOlder]);

  function getSorted(nav, rows) {
    const cfg = sortByNav[nav] || SORT_DEFAULTS[nav] || { key: 'name', dir: 'asc' };
    const factor = cfg.dir === 'asc' ? 1 : -1;
    return [...rows].sort((a, b) => factor * compareValues(a[cfg.key], b[cfg.key]));
  }

  const sortedPods = useMemo(() => getSorted('pods', filteredPods), [filteredPods, sortByNav]);
  const filteredWorkloads = useMemo(
    () => filterRowsByQuery(workloads, workloadSearch, ['kind', 'name', 'namespace', 'ready', 'age']),
    [workloads, workloadSearch]
  );
  const sortedWorkloads = useMemo(() => getSorted('workloads', filteredWorkloads), [filteredWorkloads, sortByNav]);
  const parsedNodeLabelFilter = useMemo(() => {
    const raw = nodeLabelFilter.trim();
    const [keyPart, ...valueParts] = raw.split('=');
    return {
      key: keyPart.trim(),
      hasValue: valueParts.length > 0,
      value: valueParts.join('=').trim()
    };
  }, [nodeLabelFilter]);
  const filteredNodes = useMemo(() => {
    if (!parsedNodeLabelFilter.key) return nodes;
    const key = parsedNodeLabelFilter.key.toLowerCase();
    const hasValue = parsedNodeLabelFilter.hasValue;
    const value = parsedNodeLabelFilter.value.toLowerCase();
    return nodes.filter((node) => {
      const labels = node.labels || {};
      return Object.entries(labels).some(([labelKey, labelValue]) => {
        if (labelKey.toLowerCase() !== key) return false;
        if (!hasValue) return true;
        return String(labelValue || '').toLowerCase().includes(value);
      });
    });
  }, [nodes, parsedNodeLabelFilter]);
  const availableNodeLabelKeys = useMemo(() => {
    const keys = new Set();
    nodes.forEach((node) => {
      Object.keys(node.labels || {}).forEach((key) => keys.add(key));
    });
    return Array.from(keys).sort((a, b) => a.localeCompare(b));
  }, [nodes]);
  const availableNodeLabelValues = useMemo(() => {
    if (!parsedNodeLabelFilter.key) return [];
    const values = new Set();
    nodes.forEach((node) => {
      const value = node.labels?.[parsedNodeLabelFilter.key];
      if (value != null && value !== '') {
        values.add(String(value));
      }
    });
    return Array.from(values).sort((a, b) => a.localeCompare(b));
  }, [nodes, parsedNodeLabelFilter.key]);
  const sortedNodes = useMemo(() => getSorted('nodes', filteredNodes), [filteredNodes, sortByNav]);
  const sortedEvents = useMemo(() => getSorted('events', events), [events, sortByNav]);
  const availableInsightTypes = useMemo(() => {
    const seen = new Map();
    insights.forEach((item) => {
      const key = item.check_type || item.category || 'other';
      if (!seen.has(key)) {
        seen.set(key, item.check_label || item.category || key);
      }
    });
    return Array.from(seen.entries())
      .map(([value, label]) => ({ value, label }))
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [insights]);
  useEffect(() => {
    const available = availableInsightTypes.map((item) => item.value);
    setSelectedInsightTypes((prev) => {
      if (available.length === 0) return [];
      if (prev.length === 0) return available;
      const next = prev.filter((value) => available.includes(value));
      const missing = available.filter((value) => !next.includes(value));
      return [...next, ...missing];
    });
  }, [availableInsightTypes]);
  const filteredInsightBase = useMemo(() => {
    if (selectedInsightTypes.length === 0) return [];
    return insights.filter((item) => selectedInsightTypes.includes(item.check_type || item.category || 'other'));
  }, [insights, selectedInsightTypes]);
  const insightsSummary = useMemo(() => ({
    total: filteredInsightBase.length,
    alerts: filteredInsightBase.filter((item) => item.status === 'alert').length,
    active: filteredInsightBase.filter((item) => item.status === 'alert' && !item.suppressed).length,
    passing: filteredInsightBase.filter((item) => item.status !== 'alert').length,
    suppressed: filteredInsightBase.filter((item) => item.status === 'alert' && item.suppressed).length
  }), [filteredInsightBase]);
  const visibleInsights = useMemo(() => (
    filteredInsightBase.filter((item) => {
      if (!showAllInsightChecks && item.status !== 'alert') return false;
      if (!showSuppressedInsights && item.status === 'alert' && item.suppressed) return false;
      return true;
    })
  ), [filteredInsightBase, showAllInsightChecks, showSuppressedInsights]);
  const sortedInsights = useMemo(() => getSorted('insights', visibleInsights), [visibleInsights, sortByNav]);
  const groupedInsights = useMemo(() => {
    const groups = sortedInsights.reduce((acc, alert) => {
      const key = alert.check_label || alert.category || 'Other';
      if (!acc[key]) {
        acc[key] = { label: alert.check_label || alert.category || 'Other', category: alert.category || 'Other', items: [] };
      }
      acc[key].items.push(alert);
      return acc;
    }, {});
    return Object.values(groups);
  }, [sortedInsights]);

  function isDegradedReady(ready) {
    const match = String(ready || '').match(/^(\d+)\/(\d+)$/);
    if (!match) return false;
    return Number(match[1]) < Number(match[2]);
  }

  function findLatestEvent(namespace, kind, name) {
    const target = `${kind}/${name}`.toLowerCase();
    return events.find((event) => event.namespace === namespace && String(event.object || '').toLowerCase() === target) || null;
  }

  async function findLatestEventForTarget(namespace, kind, name) {
    const existing = findLatestEvent(namespace, kind, name);
    if (existing || !api || !namespace) {
      return existing;
    }
    try {
      const eventData = await api(`/api/events?namespace=${encodeURIComponent(namespace)}&limit=200`);
      const items = eventData.items || [];
      setEvents((prev) => [
        ...prev.filter((event) => event.namespace !== namespace),
        ...items
      ]);
      const target = `${kind}/${name}`.toLowerCase();
      return items.find((event) => String(event.object || '').toLowerCase() === target) || null;
    } catch {
      return null;
    }
  }
  const filteredServices = useMemo(
    () => filterRowsByQuery(services, serviceSearch, ['name', 'namespace', 'type', 'cluster_ip', 'ports', 'age']),
    [services, serviceSearch]
  );
  const sortedServices = useMemo(() => getSorted('services', filteredServices), [filteredServices, sortByNav]);
  const sortedClusterRoles = useMemo(() => getSorted('clusterroles', clusterRoles), [clusterRoles, sortByNav]);
  const sortedClusterRoleBindings = useMemo(() => getSorted('clusterroles', clusterRoleBindings), [clusterRoleBindings, sortByNav]);
  const sortedRbacRoles = useMemo(() => getSorted('rbacroles', rbacRoles), [rbacRoles, sortByNav]);
  const sortedRoleBindings = useMemo(() => getSorted('rbacroles', roleBindings), [roleBindings, sortByNav]);
  const filteredServiceAccounts = useMemo(
    () => filterRowsByQuery(serviceAccounts, serviceAccountSearch, ['name', 'namespace', 'secrets', 'age']),
    [serviceAccounts, serviceAccountSearch]
  );
  const sortedServiceAccounts = useMemo(() => getSorted('serviceaccounts', filteredServiceAccounts), [filteredServiceAccounts, sortByNav]);
  const filteredIngresses = useMemo(
    () => filterRowsByQuery(ingresses, ingressSearch, ['name', 'namespace', 'class', 'hosts', 'address', 'age']),
    [ingresses, ingressSearch]
  );
  const sortedIngresses = useMemo(() => getSorted('ingresses', filteredIngresses), [filteredIngresses, sortByNav]);
  const filteredConfigMaps = useMemo(
    () => filterRowsByQuery(configMaps, configMapSearch, ['name', 'namespace', 'data_keys', 'age']),
    [configMaps, configMapSearch]
  );
  const sortedConfigMaps = useMemo(() => getSorted('configmaps', filteredConfigMaps), [filteredConfigMaps, sortByNav]);
  const filteredCRDs = useMemo(
    () => filterRowsByQuery(
      selectedCRDGroup ? crds.filter((item) => item.group === selectedCRDGroup) : crds,
      crdSearch,
      ['name', 'group', 'kind', 'scope', 'versions', 'age']
    ),
    [crds, crdSearch, selectedCRDGroup]
  );
  const sortedCRDs = useMemo(() => getSorted('crds', filteredCRDs), [filteredCRDs, sortByNav]);
  const selectedCRD = useMemo(() => crds.find((item) => item.name === selectedCRDName) || null, [crds, selectedCRDName]);
  const filteredCustomResources = useMemo(
    () => filterRowsByQuery(customResources, customResourceSearch, ['name', 'namespace', 'api_version', 'kind', 'age']),
    [customResources, customResourceSearch]
  );
  const sortedCustomResources = useMemo(
    () => getSorted('customresources', filteredCustomResources),
    [filteredCustomResources, sortByNav]
  );
  const filteredSecrets = useMemo(
    () => filterRowsByQuery(secrets, secretSearch, ['name', 'namespace', 'type', 'data_keys', 'age']),
    [secrets, secretSearch]
  );
  const sortedSecrets = useMemo(() => getSorted('secrets', filteredSecrets), [filteredSecrets, sortByNav]);
  const filteredPVCs = useMemo(
    () => filterRowsByQuery(pvcs, pvcSearch, ['name', 'namespace', 'status', 'volume', 'capacity', 'storage_class', 'usage', 'age']),
    [pvcs, pvcSearch]
  );
  const sortedPVCs = useMemo(() => getSorted('pvcs', filteredPVCs), [filteredPVCs, sortByNav]);
  const filteredPVs = useMemo(
    () => filterRowsByQuery(pvs, pvSearch, ['name', 'status', 'claim', 'capacity', 'storage_class', 'usage', 'age']),
    [pvs, pvSearch]
  );
  const sortedPVs = useMemo(() => getSorted('pvs', filteredPVs), [filteredPVs, sortByNav]);
  const sortedStorageClasses = useMemo(() => getSorted('storageclasses', storageClasses), [storageClasses, sortByNav]);
  const runningPodsHealth = useMemo(() => (
    healthPods
      .filter((pod) => String(pod.phase || '').toLowerCase() === 'running')
      .slice()
      .sort((a, b) => {
        if (a.namespace !== b.namespace) return String(a.namespace).localeCompare(String(b.namespace));
        return String(a.name).localeCompare(String(b.name));
      })
  ), [healthPods]);

  async function safe(action) {
    try {
      setStatus('Working...');
      await action();
      setStatus('');
    } catch (err) {
      setStatus(err.message || String(err));
    }
  }

  async function safeBottomNotice(action) {
    try {
      setStatus('');
      await action();
    } catch (err) {
      setStatus('');
      showBottomNoticeMessage('error', err.message || String(err));
    }
  }

  function toggleSort(nav, key) {
    setSortByNav((prev) => {
      const current = prev[nav] || SORT_DEFAULTS[nav] || { key: 'name', dir: 'asc' };
      if (current.key === key) {
        return { ...prev, [nav]: { key, dir: current.dir === 'asc' ? 'desc' : 'asc' } };
      }
      return { ...prev, [nav]: { key, dir: 'asc' } };
    });
  }

  function sortMark(nav, key) {
    const current = sortByNav[nav] || SORT_DEFAULTS[nav];
    const mark = !current || current.key !== key
      ? '↕'
      : (current.dir === 'asc' ? '▲' : '▼');
    return <span className="sort-mark" aria-hidden="true">{mark}</span>;
  }

  function toggleNamespace(ns) {
    setSelectedNamespaces((prev) => {
      if (prev.includes(ns)) return prev.filter((x) => x !== ns);
      return [...prev, ns];
    });
  }

  function handleNavChange(nextNav) {
    if (!nextNav) return;
    if (nextNav === 'crds') {
      setSelectedCRDName('');
      setSelectedCRDGroup('');
      setCustomResources([]);
      setCustomResourceDefinition(null);
    }
    if (nextNav === activeNav) return;
    setWarningPopover(null);
    if (warningHideTimerRef.current) {
      clearTimeout(warningHideTimerRef.current);
      warningHideTimerRef.current = null;
    }
    activateNav(nextNav);
  }

  function handleCRDSelect(name) {
    const nextName = name || '';
    setSelectedCRDName(nextName);
    const nextCRD = crds.find((item) => item.name === nextName);
    if (nextCRD) {
      const group = nextCRD.group || 'Other';
      setSelectedCRDGroup(group);
      setExpandedCRDGroups((previous) => new Set(previous).add(group));
    }
    setCustomResourceSearch('');
    setCRDNavExpanded(true);
    activateNav('crds');
    safe(() => refreshSelectedCRDResources(crds, nextName));
  }

  function handleCRDGroupSelect(group) {
    setSelectedCRDGroup(group || '');
    setSelectedCRDName('');
    setCustomResources([]);
    setCustomResourceDefinition(null);
    setCRDSearch('');
    setCRDNavExpanded(true);
    activateNav('crds');
  }

  function toggleCRDGroup(group) {
    setExpandedCRDGroups((previous) => {
      const next = new Set(previous);
      if (next.has(group)) next.delete(group);
      else next.add(group);
      return next;
    });
  }

  function toggleCRDNav() {
    const expanding = !crdNavExpanded;
    setCRDNavExpanded(expanding);
    if (expanding && crds.length === 0) {
      safe(async () => {
        const data = await api('/api/crds');
        setCRDs(data.items || []);
      });
    }
  }

  async function refreshSelectedCRDResources(crdItems = crds, crdName = selectedCRDName) {
    if (!crdName) {
      setCustomResources([]);
      setCustomResourceDefinition(null);
      return;
    }
    const selected = crdItems.find((item) => item.name === crdName);
    if (!selected) {
      setSelectedCRDName('');
      setCustomResources([]);
      setCustomResourceDefinition(null);
      return;
    }
    if (String(selected.scope).toLowerCase() === 'namespaced' && selectedNamespaces.length === 0) {
      setCustomResources([]);
      setCustomResourceDefinition({ ...selected, namespaced: true });
      return;
    }
    const params = new URLSearchParams();
    if (selectedNamespaces.length > 0) params.set('namespace', namespaceQuery);
    const suffix = params.toString() ? `?${params.toString()}` : '';
    const data = await api(`/api/crds/${encodeURIComponent(crdName)}/resources${suffix}`);
    setCustomResources(data.items || []);
    setCustomResourceDefinition(data.definition || null);
  }

  async function refreshAll() {
    if (!api) return;

    const refreshByNav = {
      pods: async () => {
        if (selectedNamespaces.length === 0) {
          setPods([]);
          return;
        }
        const podData = await api(`/api/pods?namespace=${encodeURIComponent(namespaceQuery)}`);
        setPods(podData.items || []);
      },
      workloads: async () => {
        if (selectedNamespaces.length === 0) {
          setWorkloads([]);
          return;
        }
        const workloadData = await api(`/api/workloads?namespace=${encodeURIComponent(namespaceQuery)}`);
        setWorkloads(workloadData.items || []);
      },
      nodes: async () => {
        const nodeData = await api('/api/nodes');
        setNodes(nodeData.items || []);
      },
      events: async () => {
        if (selectedNamespaces.length === 0) {
          setEvents([]);
          return;
        }
        const eventData = await api(`/api/events?namespace=${encodeURIComponent(namespaceQuery)}&limit=200`);
        setEvents(eventData.items || []);
      },
      services: async () => {
        if (selectedNamespaces.length === 0) {
          setServices([]);
          return;
        }
        const serviceData = await api(`/api/services?namespace=${encodeURIComponent(namespaceQuery)}`);
        setServices(serviceData.items || []);
      },
      clusterroles: async () => {
        const [clusterRoleData, clusterRoleBindingData] = await Promise.all([
          api('/api/clusterroles'),
          api('/api/clusterrolebindings')
        ]);
        setClusterRoles(clusterRoleData.items || []);
        setClusterRoleBindings(clusterRoleBindingData.items || []);
      },
      rbacroles: async () => {
        if (selectedNamespaces.length === 0) {
          setRbacRoles([]);
          setRoleBindings([]);
          return;
        }
        const [roleData, roleBindingData] = await Promise.all([
          api(`/api/rbac/roles?namespace=${encodeURIComponent(namespaceQuery)}`),
          api(`/api/rbac/rolebindings?namespace=${encodeURIComponent(namespaceQuery)}`)
        ]);
        setRbacRoles(roleData.items || []);
        setRoleBindings(roleBindingData.items || []);
      },
      serviceaccounts: async () => {
        if (selectedNamespaces.length === 0) {
          setServiceAccounts([]);
          return;
        }
        const data = await api(`/api/serviceaccounts?namespace=${encodeURIComponent(namespaceQuery)}`);
        setServiceAccounts(data.items || []);
      },
      ingresses: async () => {
        if (selectedNamespaces.length === 0) {
          setIngresses([]);
          return;
        }
        const ingressData = await api(`/api/ingresses?namespace=${encodeURIComponent(namespaceQuery)}`);
        setIngresses(ingressData.items || []);
      },
      configmaps: async () => {
        if (selectedNamespaces.length === 0) {
          setConfigMaps([]);
          return;
        }
        const configMapData = await api(`/api/configmaps?namespace=${encodeURIComponent(namespaceQuery)}`);
        setConfigMaps(configMapData.items || []);
      },
      crds: async () => {
        const crdData = await api('/api/crds');
        const items = crdData.items || [];
        setCRDs(items);
        await refreshSelectedCRDResources(items);
      },
      secrets: async () => {
        if (selectedNamespaces.length === 0) {
          setSecrets([]);
          return;
        }
        const secretData = await api(`/api/secrets?namespace=${encodeURIComponent(namespaceQuery)}`);
        setSecrets(secretData.items || []);
      },
      pvcs: async () => {
        if (selectedNamespaces.length === 0) {
          setPVCs([]);
          return;
        }
        const pvcData = await api(`/api/pvcs?namespace=${encodeURIComponent(namespaceQuery)}`);
        setPVCs(pvcData.items || []);
      },
      pvs: async () => {
        const pvData = await api('/api/pvs');
        setPVs(pvData.items || []);
      },
      storageclasses: async () => {
        const storageClassData = await api('/api/storageclasses');
        setStorageClasses(storageClassData.items || []);
      },
      ...Object.fromEntries(
        Object.entries(INSIGHT_NAV_CATEGORIES).map(([navId, category]) => [
          navId,
          async () => {
            await refreshInsights(category.value);
          }
        ])
      ),
      'user-management': async () => {
        if (!isAdmin) return;
        await refreshUsers();
      },
      'cluster-health': async () => {
        if (selectedNamespaces.length === 0) {
          setWorkloads([]);
          setPods([]);
          setHealthPods([]);
          setNodes([]);
          setEvents([]);
          setIngresses([]);
          setServices([]);
          setPVCs([]);
          setPVs([]);
          setStorageClasses([]);
          return;
        }
        const [
          workloadData,
          podData,
          healthPodData,
          nodeData,
          eventData,
          ingressData,
          serviceData,
          pvcData,
          pvData,
          storageClassData
        ] = await Promise.all([
          api(`/api/workloads?namespace=${encodeURIComponent(namespaceQuery)}`),
          api(`/api/pods?namespace=${encodeURIComponent(namespaceQuery)}`),
          api(`/api/pods?namespace=${encodeURIComponent(namespaceQuery)}&include_metrics=1`),
          api('/api/nodes'),
          api(`/api/events?namespace=${encodeURIComponent(namespaceQuery)}&limit=200`),
          api(`/api/ingresses?namespace=${encodeURIComponent(namespaceQuery)}`),
          api(`/api/services?namespace=${encodeURIComponent(namespaceQuery)}`),
          api(`/api/pvcs?namespace=${encodeURIComponent(namespaceQuery)}`),
          api('/api/pvs'),
          api('/api/storageclasses')
        ]);
        setWorkloads(workloadData.items || []);
        setPods(podData.items || []);
        setHealthPods(healthPodData.items || []);
        setNodes(nodeData.items || []);
        setEvents(eventData.items || []);
        setIngresses(ingressData.items || []);
        setServices(serviceData.items || []);
        setPVCs(pvcData.items || []);
        setPVs(pvData.items || []);
        setStorageClasses(storageClassData.items || []);
      },
      apply: async () => {}
    };

    const handler = refreshByNav[activeNav];
    if (handler) {
      await handler();
      return;
    }
    if (selectedNamespaces.length === 0) return;
    const podData = await api(`/api/pods?namespace=${encodeURIComponent(namespaceQuery)}`);
    setPods(podData.items || []);
  }

  async function refreshInsights(category = activeInsightCategory) {
    if (!api) return;
    const insightNamespaces = selectedNamespaces;
    if (insightNamespaces.length === 0) {
      setInsights([]);
      return;
    }
    const params = new URLSearchParams({
      namespace: insightNamespaces.join(','),
      category
    });
    const data = await api(`/api/insights?${params.toString()}`);
    setInsights(data.items || []);
  }

  function toggleInsightType(type) {
    setSelectedInsightTypes((prev) => (
      prev.includes(type)
        ? prev.filter((value) => value !== type)
        : [...prev, type]
    ));
  }

  function selectAllInsightTypes() {
    setSelectedInsightTypes(availableInsightTypes.map((item) => item.value));
  }

  function clearInsightTypes() {
    setSelectedInsightTypes([]);
  }

  function applyNodeLabelKey(labelKey) {
    if (!labelKey) return;
    setNodeLabelFilter(labelKey);
  }

  function applyNodeLabelValue(labelValue) {
    if (!parsedNodeLabelFilter.key) return;
    if (!labelValue) {
      setNodeLabelFilter(parsedNodeLabelFilter.key);
      return;
    }
    setNodeLabelFilter(`${parsedNodeLabelFilter.key}=${labelValue}`);
  }

  function selectPod(pod) {
    setSelectedPod(pod);
    activateNav('pods');
  }

  function togglePodRefSelection(pod) {
    const ref = podRefKey(pod.namespace, pod.name);
    setSelectedPodRefs((prev) => (
      prev.includes(ref)
        ? prev.filter((item) => item !== ref)
        : [...prev, ref]
    ));
  }

  function setPodRefsSelection(podsToUpdate, shouldSelect) {
    const refs = podsToUpdate.map((pod) => podRefKey(pod.namespace, pod.name));
    setSelectedPodRefs((prev) => {
      const next = new Set(prev);
      refs.forEach((ref) => {
        if (shouldSelect) {
          next.add(ref);
        } else {
          next.delete(ref);
        }
      });
      return Array.from(next);
    });
  }

  function selectedPodActionPermission(action) {
    if (selectedPods.length === 0) {
      return { allowed: false, reason: 'Select at least one pod' };
    }
    for (const pod of selectedPods) {
      const check = permissionInfo('pods', action, pod.namespace);
      if (!check.allowed) {
        return { allowed: false, reason: `${pod.namespace}/${pod.name}: ${check.reason}` };
      }
    }
    return { allowed: true, reason: '' };
  }

  function upsertTab(tab, makeActive = true) {
    setBottomTabs((prev) => {
      const idx = prev.findIndex((x) => x.id === tab.id);
      if (idx === -1) return [...prev, tab];
      const next = [...prev];
      next[idx] = { ...next[idx], ...tab };
      return next;
    });
    if (makeActive) setActiveBottomTabId(tab.id);
  }

  function closeTab(tabId) {
    if (pendingLogPrependRef.current?.tabId === tabId) {
      pendingLogPrependRef.current = null;
    }
    const ws = execSocketsRef.current[tabId];
    if (ws) {
      ws.onopen = null;
      ws.onmessage = null;
      ws.onerror = null;
      ws.onclose = null;
      delete execSocketsRef.current[tabId];
      try {
        ws.close();
      } catch {
        // ignore close errors
      }
    }
    if (execTerminalRef.current?.tabId === tabId) {
      execTerminalRef.current.dispose();
      execTerminalRef.current = null;
    }
    if (activeBottomTabId === tabId && logFollowRef.current) {
      clearInterval(logFollowRef.current);
      logFollowRef.current = null;
    }
    setBottomTabs((prev) => {
      const next = prev.filter((t) => t.id !== tabId);
      if (activeBottomTabId === tabId) {
        setActiveBottomTabId(next.length ? next[next.length - 1].id : '');
      }
      return next;
    });
  }

  function startDockResize(event) {
    if (!showBottomDock) {
      return;
    }
    event.preventDefault();
    dockResizeStateRef.current = { startedAt: Date.now() };
    document.body.classList.add('is-resizing-dock');
  }

  function showBottomNoticeMessage(type, text) {
    if (bottomNoticeTimerRef.current) {
      clearTimeout(bottomNoticeTimerRef.current);
    }
    setBottomNotice({ type, text });
    bottomNoticeTimerRef.current = window.setTimeout(() => {
      setBottomNotice(null);
      bottomNoticeTimerRef.current = null;
    }, 10000);
  }

  function scheduleLogsScrollToBottom() {
    forceLogScrollRef.current = true;
  }

  function scrollLogsToBottom() {
    const wrap = logsOutputRef.current;
    if (!wrap) return;
    wrap.scrollTop = wrap.scrollHeight;
    const content = wrap.querySelector('.logs-output');
    if (content && typeof content.scrollTop === 'number') {
      content.scrollTop = content.scrollHeight;
    }
  }

  async function loadOlderLogs(tabId) {
    const tab = bottomTabsRef.current.find((item) => item.id === tabId);
    const wrap = logsOutputRef.current;
    if (!tab || tab.type !== 'logs' || tab.loading || tab.loadingOlder || tab.canLoadOlder === false || !wrap) {
      return;
    }

    const currentTail = Math.max(Number(tab.tail || 0), 0);
    const nextTail = Math.min(Math.max(currentTail * 2, currentTail + 400, 400), 20000);
    if (nextTail <= currentTail) {
      return;
    }

    pendingLogPrependRef.current = {
      tabId,
      prevHeight: wrap.scrollHeight,
      prevTop: wrap.scrollTop
    };
    const nextTab = { ...tab, tail: nextTail, follow: false, loadingOlder: true, error: '' };
    upsertTab(nextTab, false);
    await refreshLogTab(tabId, true, {
      preserveScrollPosition: true,
      previousContent: String(tab.content || ''),
      tabOverride: nextTab
    });
  }

  function handleLogsScroll(event) {
    const wrap = event.currentTarget;
    const tab = bottomTabsRef.current.find((item) => item.id === activeBottomTabId);
    if (!tab || tab.type !== 'logs') {
      return;
    }
    if (tab.follow && !forceLogScrollRef.current && wrap.scrollTop + wrap.clientHeight < wrap.scrollHeight - 24) {
      upsertTab({ id: tab.id, follow: false }, false);
    }
    if (wrap.scrollTop <= 0) {
      void loadOlderLogs(tab.id);
    }
  }

  async function runBatchPodAction(actionLabel, podsToProcess, runner) {
    if (podsToProcess.length === 0) {
      return;
    }

    const failed = [];
    for (const pod of podsToProcess) {
      try {
        // Keep batch mutations sequential to avoid hammering the API server.
        // eslint-disable-next-line no-await-in-loop
        await runner(pod);
      } catch (err) {
        failed.push({ pod, message: err.message || String(err) });
      }
    }

    const successfulPods = podsToProcess.filter(
      (pod) => !failed.some((item) => item.pod.namespace === pod.namespace && item.pod.name === pod.name)
    );

    if (actionLabel === 'Delete') {
      const deletedRefs = new Set(
        successfulPods.map((pod) => podRefKey(pod.namespace, pod.name))
      );
      if (selectedPod && deletedRefs.has(podRefKey(selectedPod.namespace, selectedPod.name))) {
        setSelectedPod(null);
      }
    }

    setSelectedPodRefs((prev) => prev.filter((ref) => !successfulPods.some((pod) => podRefKey(pod.namespace, pod.name) === ref)));
    await refreshAll();

    if (failed.length > 0) {
      setStatus(`${actionLabel} finished with ${failed.length} error(s). First error: ${failed[0].pod.namespace}/${failed[0].pod.name}: ${failed[0].message}`);
      return;
    }

    setStatus(`${actionLabel} completed for ${podsToProcess.length} pod(s)`);
  }

  async function evictSelectedPods() {
    const podsToProcess = [...selectedPods];
    await runBatchPodAction('Evict', podsToProcess, async (pod) => {
      await evictPodByRef(pod.namespace, pod.name);
    });
  }

  async function deleteSelectedPods() {
    const podsToProcess = [...selectedPods];
    await runBatchPodAction('Delete', podsToProcess, async (pod) => {
      await deletePodByRef(pod.namespace, pod.name);
    });
  }

  function isSecretKind(kind) {
    return String(kind || '').trim().toLowerCase() === 'secret';
  }

  async function fetchManifestData(namespace, kind, name, options = {}) {
    const params = new URLSearchParams({ namespace, kind, name, _ts: String(Date.now()) });
    if (options.revealSecret) {
      params.set('reveal', '1');
    }
    return api(`/api/manifest?${params.toString()}`);
  }

  async function refreshManifestTab(tabId, tabOverride = null) {
    const tab = tabOverride || bottomTabsRef.current.find((item) => item.id === tabId);
    if (!tab || !['manifest', 'edit'].includes(tab.type)) {
      return;
    }
    upsertTab({ id: tabId, loading: true, error: '' }, false);
    try {
      const revealSecret = isSecretKind(tab.kind) && tab.type === 'manifest' && tab.secretRevealed;
      const data = await fetchManifestData(tab.namespace, tab.kind, tab.name, { revealSecret });
      const next = {
        id: tabId,
        content: data.yaml || '',
        loading: false,
        error: ''
      };
      if (isSecretKind(tab.kind) && tab.type === 'manifest') {
        next.secretDecodedData = revealSecret ? (data.decoded_data || {}) : null;
      }
      if (tab.type === 'edit') {
        next.originalContent = data.yaml || '';
      }
      upsertTab(next, false);
    } catch (err) {
      upsertTab({ id: tabId, loading: false, error: err.message || String(err) }, false);
    }
  }

  async function openManifestTab(namespace, kind, name) {
    const ns = namespace || primaryNamespace || 'default';
    const id = `manifest:${bottomTabResourceKey(ns, kind, name)}`;
    const title = `View ${displayKind(kind)}/${name}`;
    if (bottomTabsRef.current.some((tab) => tab.id === id)) {
      setActiveBottomTabId(id);
      return;
    }
    const tab = { id, type: 'manifest', title, content: '', namespace: ns, kind, name, secretRevealed: false, secretDecodedData: null, loading: true, error: '' };
    upsertTab(tab);
    await refreshManifestTab(id, tab);
  }

  async function revealSecretManifestTab(tabId) {
    const tab = bottomTabsRef.current.find((item) => item.id === tabId);
    if (!tab || tab.type !== 'manifest' || !isSecretKind(tab.kind)) {
      return;
    }
    upsertTab({ id: tabId, loading: true, error: '' }, false);
    try {
      const data = await fetchManifestData(tab.namespace, tab.kind, tab.name, { revealSecret: true });
      upsertTab({
        id: tabId,
        content: data.yaml || '',
        secretRevealed: true,
        secretDecodedData: data.decoded_data || {},
        loading: false,
        error: ''
      }, false);
    } catch (err) {
      upsertTab({ id: tabId, loading: false }, false);
      throw err;
    }
  }

  async function openNodePodsTab(nodeName) {
    if (!nodeName) throw new Error('Node name is required');
    if (selectedNamespaces.length === 0) throw new Error('Select namespace first');
    const id = `node-pods:${nodeName}`;
    const title = `Pods on ${nodeName}`;
    upsertTab({ id, type: 'node-pods', title, nodeName, items: [], loading: true, error: '' });
    try {
      const data = await api(`/api/pods?namespace=${encodeURIComponent(namespaceQuery)}`);
      const items = (data.items || [])
        .filter((pod) => pod.node === nodeName)
        .sort((a, b) => {
          if (a.namespace !== b.namespace) return String(a.namespace).localeCompare(String(b.namespace));
          return String(a.name).localeCompare(String(b.name));
        });
      upsertTab({ id, type: 'node-pods', title, nodeName, items, loading: false, error: '' });
    } catch (err) {
      upsertTab({ id, type: 'node-pods', title, nodeName, items: [], loading: false, error: err.message || String(err) });
    }
  }

  async function openEditTab(namespace, kind, name) {
    const ns = namespace || primaryNamespace || 'default';
    const id = `edit:${bottomTabResourceKey(ns, kind, name)}`;
    const title = `Edit ${displayKind(kind)}/${name}`;
    if (bottomTabsRef.current.some((tab) => tab.id === id)) {
      setActiveBottomTabId(id);
      return;
    }
    const tab = { id, type: 'edit', title, content: '', namespace: ns, kind, name, loading: true, error: '' };
    upsertTab(tab);
    await refreshManifestTab(id, tab);
  }

  async function applyEditTab(tabId, dryRun) {
    const tab = bottomTabsRef.current.find((t) => t.id === tabId);
    if (!tab) return;
    if (!primaryNamespace && !tab.namespace) throw new Error('Select namespace first');

    upsertTab({ id: tabId, loading: true, error: '' }, false);
    try {
      const data = await api('/api/manifest', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          namespace: tab.namespace || primaryNamespace,
          kind: tab.kind,
          name: tab.name,
          yaml: tab.content,
          dryRun
        })
      });
      upsertTab({ id: tabId, loading: false }, false);
      showBottomNoticeMessage('success', summarizeApplyResult(data, dryRun));
      if (!dryRun) {
        await refreshManifestTab(tabId);
      }
      await refreshAll();
    } catch (err) {
      upsertTab({ id: tabId, loading: false }, false);
      showBottomNoticeMessage('error', err.message || String(err));
    }
  }

  async function openPodLogsTab(namespace, podName, container = '', containers = []) {
    const knownPod = pods.find((item) => item.namespace === namespace && item.name === podName);
    const containerOptions = normalizeContainerNames(
      Array.isArray(containers) && containers.length > 0 ? containers : knownPod?.containers
    );
    const selectedContainer = String(container || containerOptions[0] || '').trim();
    const id = `logs:pod:${namespace}:${podName}`;
    const title = `Logs Pod/${podName}`;
    const tab = {
      id,
      type: 'logs',
      logKind: 'pod',
      title,
      content: '',
      loading: true,
      error: '',
      follow: true,
      search: '',
      showWarnings: false,
      showErrors: false,
      namespace,
      pod: podName,
      container: selectedContainer,
      containers: containerOptions,
      tail: 400,
      canLoadOlder: true,
      loadingOlder: false
    };
    upsertTab(tab);
    await refreshLogTab(id, false, { forceScrollToBottom: true, tabOverride: tab });
  }

  async function openWorkloadLogsTab(namespace, kind, name) {
    const id = `logs:workload:${namespace}:${kind}:${name}`;
    const title = `Logs ${displayKind(kind)}/${name}`;
    const tab = {
      id,
      type: 'logs',
      logKind: 'workload',
      title,
      content: '',
      loading: true,
      error: '',
      follow: true,
      search: '',
      showWarnings: false,
      showErrors: false,
      namespace,
      kind,
      name,
      tail: 300,
      canLoadOlder: true,
      loadingOlder: false
    };
    upsertTab(tab);
    await refreshLogTab(id, false, { forceScrollToBottom: true, tabOverride: tab });
  }

  async function refreshLogTab(tabId, silent = false, options = {}) {
    const { forceScrollToBottom = false, preserveScrollPosition = false, previousContent = '', tabOverride = null } = options;
    const tab = tabOverride || bottomTabsRef.current.find((t) => t.id === tabId) || (activeBottomTab?.id === tabId ? activeBottomTab : null);
    if (!tab || tab.type !== 'logs') return;
    if (forceScrollToBottom) {
      scheduleLogsScrollToBottom();
    }
    if (!silent) {
      upsertTab({ id: tabId, loading: true, error: '' }, false);
    }
    try {
      let text = '';
      if (tab.logKind === 'pod') {
        const containers = normalizeContainerNames(tab.containers);
        const container = tab.container || containers[0] || '';
        const params = new URLSearchParams({
          namespace: tab.namespace,
          pod: tab.pod,
          tail: String(tab.tail || 400),
          _ts: String(Date.now())
        });
        if (container) {
          params.set('container', container);
        }
        text = await api(`/api/podlogs?${params.toString()}`);
        if (container && container !== tab.container) {
          upsertTab({ id: tabId, container }, false);
        }
      } else {
        const params = new URLSearchParams({
          namespace: tab.namespace,
          kind: tab.kind,
          name: tab.name,
          tail: String(tab.tail || 300),
          _ts: String(Date.now())
        });
        text = await api(`/api/workloadlogs?${params.toString()}`);
      }
      if (tab.follow) {
        scheduleLogsScrollToBottom();
      }
      upsertTab({
        id: tabId,
        content: text,
        loading: false,
        loadingOlder: false,
        canLoadOlder: preserveScrollPosition ? text !== previousContent : tab.canLoadOlder,
        error: ''
      }, false);
    } catch (err) {
      pendingLogPrependRef.current = null;
      upsertTab({ id: tabId, loading: false, loadingOlder: false, error: err.message || String(err) }, false);
    }
  }

  async function changeLogContainer(tabId, container) {
    const tab = bottomTabsRef.current.find((item) => item.id === tabId);
    if (!tab || tab.type !== 'logs' || tab.logKind !== 'pod') {
      return;
    }
    const nextTab = {
      ...tab,
      container,
      content: '',
      loading: true,
      error: '',
      tail: 400,
      canLoadOlder: true,
      loadingOlder: false
    };
    upsertTab(nextTab, false);
    await refreshLogTab(tabId, false, {
      forceScrollToBottom: true,
      tabOverride: nextTab
    });
  }

  function appendExecOutput(tabId, chunk) {
    if (execTerminalRef.current?.tabId === tabId) {
      execTerminalRef.current.term.write(chunk || '');
    }
    setBottomTabs((prev) => prev.map((t) => (
      t.id === tabId
        ? { ...t, content: `${t.content || ''}${chunk || ''}` }
        : t
    )));
  }

  function openPodExecTab(namespace, podName, container = '', containers = []) {
    const knownPod = pods.find((item) => item.namespace === namespace && item.name === podName);
    const containerOptions = normalizeContainerNames(
      Array.isArray(containers) && containers.length > 0 ? containers : knownPod?.containers
    );
    const selectedContainer = String(container || containerOptions[0] || '').trim();
    const id = `exec:pod:${namespace}:${podName}:${selectedContainer || '-'}`;
    const title = `Exec ${namespace}/${podName}${selectedContainer ? `:${selectedContainer}` : ''}`;
    const existing = execSocketsRef.current[id];
    if (existing && existing.readyState === WebSocket.OPEN) {
      setActiveBottomTabId(id);
      return;
    }

    upsertTab({
      id,
      type: 'exec',
      title,
      content: '',
      namespace,
      pod: podName,
      container: selectedContainer,
      containers: containerOptions,
      connected: false,
      loading: false,
      error: ''
    });

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const params = new URLSearchParams({ namespace, pod: podName, token, username });
    if (selectedContainer) {
      params.set('container', selectedContainer);
    }
    const wsURL = `${protocol}//${window.location.host}${withBasePath(`/api/pods/exec/ws?${params.toString()}`)}`;
    const ws = new WebSocket(wsURL);
    execSocketsRef.current[id] = ws;

    ws.onopen = () => {
      if (execSocketsRef.current[id] !== ws) return;
      upsertTab({ id, connected: true, error: '' }, false);
      if (execTerminalRef.current?.tabId === id) {
        sendExecResize(id, execTerminalRef.current.term.cols, execTerminalRef.current.term.rows);
      }
    };
    ws.onmessage = (event) => {
      if (execSocketsRef.current[id] !== ws) return;
      appendExecOutput(id, String(event.data || ''));
    };
    ws.onerror = () => {
      if (execSocketsRef.current[id] !== ws) return;
      upsertTab({ id, connected: false, error: 'Exec websocket error' }, false);
    };
    ws.onclose = () => {
      if (execSocketsRef.current[id] !== ws) return;
      delete execSocketsRef.current[id];
      upsertTab({ id, connected: false }, false);
    };
  }

  function sendExecData(tabId, data) {
    const ws = execSocketsRef.current[tabId];
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      upsertTab({ id: tabId, error: 'Exec session is not connected' }, false);
      return;
    }
    ws.send(data);
  }

  function sendExecResize(tabId, cols, rows) {
    const ws = execSocketsRef.current[tabId];
    const safeCols = Math.max(2, Math.floor(Number(cols) || 0));
    const safeRows = Math.max(1, Math.floor(Number(rows) || 0));
    if (!ws || ws.readyState !== WebSocket.OPEN || safeCols <= 0 || safeRows <= 0) {
      return;
    }
    const payload = JSON.stringify({ type: 'resize', cols: safeCols, rows: safeRows });
    ws.send(new TextEncoder().encode(payload));
  }

  async function scaleWorkload() {
    const targetNamespace = deploymentNamespace || primaryNamespace;
    if (!targetNamespace) throw new Error('Select namespace first');
    const targetKind = String(scaleTargetKind || 'Deployment');
    const scalePath = targetKind === 'StatefulSet' ? '/api/statefulsets/scale' : '/api/deployments/scale';
    await api(scalePath, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ namespace: targetNamespace, name: deploymentName, replicas: Number(replicas) })
    });
    setShowScaleModal(false);
    await refreshAll();
  }

  async function restartDeployment() {
    const targetNamespace = deploymentNamespace || primaryNamespace;
    if (!targetNamespace) throw new Error('Select namespace first');
    await api('/api/deployments/restart', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ namespace: targetNamespace, name: deploymentName })
    });
    await refreshAll();
  }

  function openScaleModal(workload) {
    setScaleTargetKind(String(workload.kind || 'Deployment'));
    setDeploymentName(workload.name || '');
    setDeploymentNamespace(workload.namespace || '');
    setReplicas(desiredReplicasFromReady(workload.ready));
    setShowScaleModal(true);
  }

  async function deletePodByRef(namespace, name) {
    await api('/api/pods/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ namespace, name })
    });
  }

  async function evictPodByRef(namespace, name) {
    await api('/api/pods/evict', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ namespace, name })
    });
  }

  async function deleteResourceByRef(kind, namespace, name) {
    await api('/api/resources/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ kind, namespace, name })
    });
  }

  function openDrainModal(nodeName) {
    setDrainTargetNode(nodeName || '');
    setDrainForce(false);
    setShowDrainModal(true);
  }

  async function drainNodeAction(nodeName) {
    if (!nodeName) throw new Error('Node name is required');
    setShowDrainModal(false);
    const data = await api('/api/nodes/drain', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: nodeName, force: drainForce })
    });
    const result = data.result || {};
    const baseMessage = `Drain ${nodeName}: evicted ${data.evicted ?? result.evicted?.length ?? 0}, skipped ${data.skipped ?? result.skipped?.length ?? 0}, failed ${data.failed ?? result.failed?.length ?? 0}`;
    setStatus(data.warning ? `${baseMessage} (${data.warning})` : baseMessage);
    await refreshAll();
  }

  async function uncordonNodeAction(nodeName) {
    if (!nodeName) throw new Error('Node name is required');
    await api('/api/nodes/uncordon', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: nodeName })
    });
    setStatus(`Node ${nodeName} is schedulable again`);
    await refreshAll();
  }

  async function applyYaml(dryRun) {
    if (!primaryNamespace) {
      showBottomNoticeMessage('error', 'Select namespace first');
      return;
    }
    try {
      const payload = { namespace: primaryNamespace, yaml: yamlText, dryRun, fieldManager: 'beaverdeck-ui' };
      const data = await api('/api/apply', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      showBottomNoticeMessage('success', summarizeApplyResult(data, dryRun));
      await refreshAll();
    } catch (err) {
      showBottomNoticeMessage('error', err.message || String(err));
    }
  }

  function loadTemplate(name) {
    setSelectedTemplate(name);
    if (name && APPLY_TEMPLATES[name]) {
      setYamlText(APPLY_TEMPLATES[name]);
    }
  }

  async function refreshUsers() {
    if (!api || !isAdmin) return;
    const [usersData, rolesData, googleConfigData, googleMappingsData, oidcConfigData, oidcMappingsData] = await Promise.all([
      api('/api/admin/users'),
      api('/api/admin/roles'),
      api('/api/admin/google/config'),
      api('/api/admin/google/mappings'),
      api('/api/admin/oidc/config'),
      api('/api/admin/oidc/mappings')
    ]);
    const roles = rolesData.items || [];
    setManagedUsers(usersData.items || []);
    setManagedRoles(roles);
    setGoogleConfig({
      client_id: googleConfigData.client_id || '',
      client_secret: googleConfigData.client_secret || '',
      hosted_domain: googleConfigData.hosted_domain || '',
      service_account_json: googleConfigData.service_account_json || '',
      delegated_admin_email: googleConfigData.delegated_admin_email || ''
    });
    setGoogleMappings(googleMappingsData.items || []);
    setOIDCConfig(normalizeOIDCConfig(oidcConfigData));
    setOIDCMappings(oidcMappingsData.items || []);
    if (roles.length > 0 && !roles.find((r) => r.name === newUserRole)) {
      setNewUserRole(roles[0].name);
    }
    if (roles.length > 0 && !roles.find((r) => r.name === newGoogleRole)) {
      setNewGoogleRole(roles[0].name);
    }
    if (roles.length > 0 && !roles.find((r) => r.name === newOIDCRole)) {
      setNewOIDCRole(roles[0].name);
    }
  }

  async function exportAdminConfig() {
	const data = await api('/api/admin/config/export');
	const stamp = new Date().toISOString().replace(/[:.]/g, '-');
	const blob = new Blob([String(data)], { type: 'application/yaml' });
	const url = URL.createObjectURL(blob);
	const link = document.createElement('a');
	link.href = url;
	link.download = `beaverdeck-config-${stamp}.yaml`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }

  function openConfigImportPicker() {
    configImportInputRef.current?.click();
  }

  async function importAdminConfigFile(file) {
    if (!file) return;
    if (!window.confirm('Import BeaverDeck configuration and require sign-in again?')) {
      if (configImportInputRef.current) {
        configImportInputRef.current.value = '';
      }
      return;
    }
    try {
      const text = await file.text();
      await api('/api/admin/config/import', {
        method: 'POST',
        headers: { 'Content-Type': 'application/yaml' },
        body: text
      });
      await reloadAuthProviders();
    } finally {
      if (configImportInputRef.current) {
        configImportInputRef.current.value = '';
      }
    }
    await logout({ revokeRemote: false, reason: 'Configuration imported. Sign in again.' });
  }

  async function createUser() {
    if (!newUsername.trim() || !newUserPassword.trim()) throw new Error('username and password are required');
    await api('/api/admin/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: newUsername.trim(),
        password: newUserPassword.trim(),
        role: newUserRole
      })
    });
    setNewUsername('');
    setNewUserPassword('');
    setNewUserRole('viewer');
    setShowCreateUserModal(false);
    await refreshUsers();
  }

  function openCreateUserModal() {
    setNewUsername('');
    setNewUserPassword('');
    setNewUserRole(managedRoles[0]?.name || 'viewer');
    setShowCreateUserModal(true);
  }

  async function updateUserRole(username, role) {
    await api('/api/admin/users/role', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, role })
    });
    await refreshUsers();
  }

  async function deleteUser(username) {
    await api('/api/admin/users/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username })
    });
    await refreshUsers();
  }

  async function resetLocalUserPassword(username) {
    setPasswordPromptUsername(username);
    setPasswordPromptValue('');
    setPasswordPromptConfirm('');
    setShowPasswordPromptModal(true);
  }

  async function applyPasswordReset() {
    if (!passwordPromptUsername.trim()) throw new Error('username is required');
    if (!passwordPromptValue.trim()) throw new Error('password is required');
    if (passwordPromptValue !== passwordPromptConfirm) throw new Error('passwords do not match');
    await api('/api/admin/users/password-reset', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: passwordPromptUsername, password: passwordPromptValue.trim() })
    });
    setShowPasswordPromptModal(false);
    setPasswordPromptUsername('');
    setPasswordPromptValue('');
    setPasswordPromptConfirm('');
    await refreshUsers();
  }

  async function saveGoogleConfig() {
    await api('/api/admin/google/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(googleConfig)
    });
    await reloadAuthProviders();
    await refreshUsers();
  }

  async function testGoogleConfig() {
    const data = await api('/api/admin/google/config/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(googleConfig)
    });
    showBottomNoticeMessage('success', data.message || 'Google config test passed');
  }

  async function disableGoogleAuth() {
    if (!window.confirm('Disable Google auth and remove all Google group mappings?')) {
      return;
    }
    await api('/api/admin/google/reset', { method: 'POST' });
    setShowGoogleConfigModal(false);
    setShowGoogleMappingsModal(false);
    resetGoogleMappingForm();
    await reloadAuthProviders();
    await refreshUsers();
    showBottomNoticeMessage('success', 'Google auth has been disabled');
  }

  async function saveOIDCConfig() {
    const nextConfig = normalizeOIDCConfig(oidcConfigDraft, oidcConfigModalMode === 'entra' ? DEFAULT_ENTRA_CONFIG : DEFAULT_OIDC_CONFIG);
    await api('/api/admin/oidc/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(nextConfig)
    });
    setOIDCConfig(nextConfig);
    await reloadAuthProviders();
    await refreshUsers();
  }

  function openOIDCConfigModal() {
    setOIDCConfigDraft(normalizeOIDCConfig(isEntraOIDCConfig(oidcConfig) ? DEFAULT_OIDC_CONFIG : oidcConfig, DEFAULT_OIDC_CONFIG));
    setOIDCConfigModalMode('oidc');
    setShowOIDCConfigModal(true);
  }

  function openEntraConfigModal() {
    setOIDCConfigDraft(normalizeOIDCConfig(isEntraOIDCConfig(oidcConfig) ? oidcConfig : DEFAULT_ENTRA_CONFIG, DEFAULT_ENTRA_CONFIG));
    setOIDCConfigModalMode('entra');
    setShowOIDCConfigModal(true);
  }

  async function testOIDCConfig() {
    const testConfig = normalizeOIDCConfig(oidcConfigDraft, oidcConfigModalMode === 'entra' ? DEFAULT_ENTRA_CONFIG : DEFAULT_OIDC_CONFIG);
    const data = await api('/api/admin/oidc/config/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(testConfig)
    });
    showBottomNoticeMessage('success', data.message || 'OpenID Connect config test passed');
  }

  async function disableOIDCAuth() {
    if (!window.confirm('Disable OpenID Connect and remove all OpenID Connect group mappings?')) {
      return;
    }
    await api('/api/admin/oidc/reset', { method: 'POST' });
    setShowOIDCConfigModal(false);
    setShowOIDCMappingsModal(false);
    resetOIDCMappingForm();
    await reloadAuthProviders();
    await refreshUsers();
    showBottomNoticeMessage('success', 'OpenID Connect has been disabled');
  }

  async function saveGoogleMapping() {
    const groupEmail = editingGoogleGroupEmail || newGoogleGroupEmail.trim();
    if (!groupEmail) throw new Error('google group email is required');
    await api('/api/admin/google/mappings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ group_email: groupEmail, role: newGoogleRole })
    });
    setEditingGoogleGroupEmail('');
    setNewGoogleGroupEmail('');
    setNewGoogleRole(managedRoles[0]?.name || 'viewer');
    await refreshUsers();
  }

  async function deleteGoogleMapping(groupEmail) {
    await api('/api/admin/google/mappings/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ group_email: groupEmail })
    });
    if (editingGoogleGroupEmail === groupEmail) {
      resetGoogleMappingForm();
    }
    await refreshUsers();
  }

  function editGoogleMapping(item) {
    setEditingGoogleGroupEmail(item.group_email || '');
    setNewGoogleGroupEmail(item.group_email || '');
    setNewGoogleRole(item.role || managedRoles[0]?.name || 'viewer');
  }

  function resetGoogleMappingForm() {
    setEditingGoogleGroupEmail('');
    setNewGoogleGroupEmail('');
    setNewGoogleRole(managedRoles[0]?.name || 'viewer');
  }

  async function saveOIDCMapping() {
    const groupName = editingOIDCGroupName || newOIDCGroupName.trim();
    if (!groupName) throw new Error('OpenID Connect group is required');
    await api('/api/admin/oidc/mappings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ group_name: groupName, role: newOIDCRole })
    });
    setEditingOIDCGroupName('');
    setNewOIDCGroupName('');
    setNewOIDCRole(managedRoles[0]?.name || 'viewer');
    await refreshUsers();
  }

  async function deleteOIDCMapping(groupName) {
    await api('/api/admin/oidc/mappings/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ group_name: groupName })
    });
    if (editingOIDCGroupName === groupName) {
      resetOIDCMappingForm();
    }
    await refreshUsers();
  }

  function editOIDCMapping(item) {
    setEditingOIDCGroupName(item.group_name || '');
    setNewOIDCGroupName(item.group_name || '');
    setNewOIDCRole(item.role || managedRoles[0]?.name || 'viewer');
  }

  function resetOIDCMappingForm() {
    setEditingOIDCGroupName('');
    setNewOIDCGroupName('');
    setNewOIDCRole(managedRoles[0]?.name || 'viewer');
  }

  function openCreateRoleModal() {
    setEditingRoleName('');
    setRoleFormName('');
    setRoleFormMode('viewer');
    setRoleFormNamespaces([]);
    setRoleFormPermissions(defaultRolePermissions().resources);
    setShowRoleModal(true);
  }

  function openEditRoleModal(role) {
    const perms = normalizeRolePermissions(role.permissions);
    setEditingRoleName(role.name);
    setRoleFormName(role.name);
    setRoleFormMode(role.mode || 'viewer');
    setRoleFormNamespaces(perms.namespaces || []);
    setRoleFormPermissions(perms.resources || defaultRolePermissions().resources);
    setShowRoleModal(true);
  }

  function toggleRoleNamespace(ns) {
    setRoleFormNamespaces((prev) => (prev.includes(ns) ? prev.filter((x) => x !== ns) : [...prev, ns]));
  }

  function setRolePermissionLevel(resource, level) {
    const resolved = resolveRoleLevel(resource, level);
    setRoleFormPermissions((prev) => ({
      ...prev,
      [resource]: permissionFlags(resolved)
    }));
  }

  function setAllRolePermissionLevels(level) {
    setRoleFormPermissions(() => {
      const next = {};
      ROLE_RESOURCES.forEach((resource) => {
        next[resource] = permissionFlags(resolveRoleLevel(resource, level));
      });
      return next;
    });
  }

  async function saveRole() {
    const name = roleFormName.trim().toLowerCase();
    if (!name) throw new Error('role name is required');
    const payload = {
      name,
      mode: roleFormMode,
      permissions: roleFormMode === 'admin' ? {} : {
        namespaces: roleFormNamespaces,
        resources: roleFormPermissions
      }
    };
    const path = editingRoleName ? '/api/admin/roles/update' : '/api/admin/roles';
    await api(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    setShowRoleModal(false);
    setEditingRoleName('');
    await refreshUsers();
  }

  async function deleteRole(name) {
    await api('/api/admin/roles/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name })
    });
    await refreshUsers();
  }

  async function setInsightSuppressed(key, suppressed) {
    await api('/api/insights/suppress', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key, suppressed })
    });
    await refreshInsights();
  }

  async function loadPodWarningDetails(pod) {
    const cacheKey = `pod:${pod.namespace}:${pod.name}`;
    if (warningCache[cacheKey]) {
      return warningCache[cacheKey];
    }
    const shouldPreferLogs = String(pod.phase) === 'Running' || Number(pod.restarts || 0) > 0;

    let payload;
    if (shouldPreferLogs) {
      try {
        const params = new URLSearchParams({ namespace: pod.namespace, pod: pod.name, tail: '40' });
        const text = await api(`/api/podlogs?${params.toString()}`);
        payload = {
          kind: 'logs',
          title: 'Recent logs',
          text: text || 'No logs available'
        };
      } catch {
        const relatedEvent = await findLatestEventForTarget(pod.namespace, 'pod', pod.name);
        payload = relatedEvent ? {
          kind: 'event',
          title: relatedEvent.reason || 'Pod event',
          text: relatedEvent.message || 'No details'
        } : {
          kind: 'event',
          title: 'No diagnostic data',
          text: 'No matching event or logs were found.'
        };
      }
    } else {
      const relatedEvent = await findLatestEventForTarget(pod.namespace, 'pod', pod.name);
      if (relatedEvent) {
        payload = {
          kind: 'event',
          title: relatedEvent.reason || 'Pod event',
          text: relatedEvent.message || 'No details'
        };
      } else {
        try {
          const params = new URLSearchParams({ namespace: pod.namespace, pod: pod.name, tail: '40' });
          const text = await api(`/api/podlogs?${params.toString()}`);
          payload = {
            kind: 'logs',
            title: 'Recent logs',
            text: text || 'No logs available'
          };
        } catch {
          payload = {
            kind: 'event',
            title: 'No diagnostic data',
            text: 'No matching event or logs were found.'
          };
        }
      }
    }

    setWarningCache((prev) => ({ ...prev, [cacheKey]: payload }));
    return payload;
  }

  async function loadWorkloadWarningDetails(workload) {
    const cacheKey = `workload:${workload.namespace}:${workload.kind}:${workload.name}`;
    if (warningCache[cacheKey]) {
      return warningCache[cacheKey];
    }
    const relatedEvent = await findLatestEventForTarget(workload.namespace, workload.kind, workload.name);
    let payload;
    if (relatedEvent) {
      payload = {
        kind: 'event',
        title: relatedEvent.reason || 'Workload event',
        text: relatedEvent.message || 'No details'
      };
    } else {
      try {
        const params = new URLSearchParams({ namespace: workload.namespace, kind: workload.kind, name: workload.name, tail: '40' });
        const text = await api(`/api/workloadlogs?${params.toString()}`);
        payload = {
          kind: 'logs',
          title: 'Recent workload logs',
          text: text || 'No logs available'
        };
      } catch {
        payload = {
          kind: 'event',
          title: 'No diagnostic data',
          text: 'No matching workload event or logs were found.'
        };
      }
    }
    setWarningCache((prev) => ({ ...prev, [cacheKey]: payload }));
    return payload;
  }

  async function showWarningPopover(event, target) {
    if (warningHideTimerRef.current) {
      clearTimeout(warningHideTimerRef.current);
      warningHideTimerRef.current = null;
    }
    const rect = event.currentTarget.getBoundingClientRect();
    const base = {
      key: target.key,
      top: rect.bottom + 8,
      left: Math.min(rect.left, window.innerWidth - 420),
      title: 'Loading diagnostics...',
      text: '',
      loading: true
    };
    setWarningPopover(base);
    const payload = target.type === 'pod'
      ? await loadPodWarningDetails(target.item)
      : await loadWorkloadWarningDetails(target.item);
    setWarningPopover((current) => {
      if (!current || current.key !== target.key) return current;
      return {
        ...current,
        title: payload.title,
        text: payload.text,
        loading: false
      };
    });
  }

  function scheduleWarningPopoverHide(key) {
    if (warningHideTimerRef.current) {
      clearTimeout(warningHideTimerRef.current);
    }
    warningHideTimerRef.current = setTimeout(() => {
      setWarningPopover((current) => (current?.key === key ? null : current));
      warningHideTimerRef.current = null;
    }, 180);
  }

  function cancelWarningPopoverHide() {
    if (warningHideTimerRef.current) {
      clearTimeout(warningHideTimerRef.current);
      warningHideTimerRef.current = null;
    }
  }

  function navForResourceKind(kind) {
    switch (String(kind || '').toLowerCase()) {
      case 'pod':
        return 'pods';
      case 'node':
        return 'nodes';
      case 'deployment':
      case 'statefulset':
      case 'daemonset':
      case 'replicaset':
      case 'job':
      case 'cronjob':
        return 'workloads';
      case 'service':
        return 'services';
      case 'ingress':
        return 'ingresses';
      case 'configmap':
        return 'configmaps';
      case 'secret':
        return 'secrets';
      case 'persistentvolumeclaim':
      case 'pvc':
        return 'pvcs';
      case 'persistentvolume':
      case 'pv':
        return 'pvs';
      case 'storageclass':
        return 'storageclasses';
      case 'customresourcedefinition':
      case 'crd':
        return 'crds';
      case 'clusterrole':
      case 'clusterrolebinding':
        return 'clusterroles';
      case 'role':
      case 'rolebinding':
        return 'rbacroles';
      case 'serviceaccount':
        return 'serviceaccounts';
      default:
        return 'pods';
    }
  }

  function openInsightResource(alert) {
    if (!alert?.resource_kind || !alert?.resource_name) return;
    const kind = String(alert.resource_kind).toLowerCase();
    activateNav(navForResourceKind(kind));
    if (kind === 'pod') {
      safe(() => openManifestTab(alert.namespace || primaryNamespace, 'pod', alert.resource_name));
      return;
    }
    if (kind === 'node') {
      safe(() => openManifestTab(primaryNamespace, 'node', alert.resource_name));
      return;
    }
    safe(() => openManifestTab(alert.namespace || primaryNamespace, kind, alert.resource_name));
  }

  function openInsightLogs(alert) {
    if (!alert?.resource_kind || !alert?.resource_name) return;
    const kind = String(alert.resource_kind).toLowerCase();
    if (kind === 'pod') {
      activateNav('pods');
      safe(() => openPodLogsTab(alert.namespace || primaryNamespace, alert.resource_name));
      return;
    }
    if (['deployment', 'statefulset', 'daemonset'].includes(kind)) {
      activateNav('workloads');
      safe(() => openWorkloadLogsTab(alert.namespace || primaryNamespace, alert.resource_kind, alert.resource_name));
    }
  }

  if (authBootstrapping) {
    return (
      <LoginScreen
        title=""
        message="Loading authentication..."
        usernameInput=""
        setUsernameInput={() => {}}
        passwordInput=""
        setPasswordInput={() => {}}
        login={() => {}}
        authProviders={authProviders}
        startGoogleLogin={startGoogleLogin}
        startOIDCLogin={startOIDCLogin}
        authError=""
        showInputs={false}
        appVersion={authProviders.appVersion || ''}
      />
    );
  }

  if (!bootstrapState.initialized) {
    return (
      <BootstrapSetupScreen
        adminUsername={bootstrapAdminUsername}
        setAdminUsername={setBootstrapAdminUsername}
        adminPassword={bootstrapAdminPassword}
        setAdminPassword={setBootstrapAdminPassword}
        adminPasswordConfirm={bootstrapAdminPasswordConfirm}
        setAdminPasswordConfirm={setBootstrapAdminPasswordConfirm}
        onComplete={() => {
          void (async () => {
            try {
              setBootstrapError('');
              if (!bootstrapAdminUsername.trim()) throw new Error('admin username is required');
              if (!bootstrapAdminPassword.trim()) throw new Error('admin password is required');
              if (bootstrapAdminPassword !== bootstrapAdminPasswordConfirm) throw new Error('admin passwords do not match');
              await completeBootstrap(bootstrapAdminUsername, bootstrapAdminPassword);
              setBootstrapAdminPassword('');
              setBootstrapAdminPasswordConfirm('');
            } catch (err) {
              setBootstrapError(err.message || String(err));
            }
          })();
        }}
        statusText=""
        errorText={bootstrapError}
      />
    );
  }

  if (!isLoggedIn) {
    return (
      <LoginScreen
        title=""
        message=""
        usernameInput={usernameInput}
        setUsernameInput={setUsernameInput}
        passwordInput={passwordInput}
        setPasswordInput={setPasswordInput}
        login={login}
        authProviders={authProviders}
        startGoogleLogin={startGoogleLogin}
        startOIDCLogin={startOIDCLogin}
        authError={authError}
        appVersion={authProviders.appVersion || ''}
      />
    );
  }

  return (
    <div className="app-shell">
      <SidebarNav
        activeNav={activeNav}
        clusterName={currentUser.clusterName}
        selectedNamespaces={selectedNamespaces}
        namespaces={namespaces}
        setSelectedNamespaces={setSelectedNamespaces}
        toggleNamespace={toggleNamespace}
        visibleMenu={visibleMenu}
        expandedNavSections={expandedNavSections}
        toggleNavSection={toggleNavSection}
        handleNavChange={handleNavChange}
        selectedCRDName={selectedCRDName}
        selectedCRDGroup={selectedCRDGroup}
        crdNavExpanded={crdNavExpanded}
        expandedCRDGroups={expandedCRDGroups}
        toggleCRDNav={toggleCRDNav}
        toggleCRDGroup={toggleCRDGroup}
        handleCRDGroupSelect={handleCRDGroupSelect}
        handleCRDSelect={handleCRDSelect}
      />

      <section className="workspace">
        <WorkspaceHeader
          title={visibleNavItems.find((x) => x.id === activeNav)?.label || 'BeaverDeck'}
          status={status}
          onRefresh={() => safe(refreshAll)}
          onProfile={() => setShowProfile(true)}
        />

        <div className="workspace-main" ref={workspaceMainRef}>
        <div
          className={`content-panel ${showBottomDock ? 'content-panel-docked' : ''}`}
          style={showBottomDock ? { flex: `0 0 ${(dockSplitRatio * 100).toFixed(1)}%` } : undefined}
        >
          <div key={activeNav} className={`nav-view ${isPodsView ? 'nav-view-pods' : ''}`}>
          {showInitialNavLoader ? (
            <div className="nav-loading-state">
              <strong>Loading {visibleNavItems.find((x) => x.id === activeNav)?.label || 'view'}...</strong>
              <span>Fetching data for this section.</span>
            </div>
          ) : (
            <>
          {activeNav === 'pods' && (
            <PodsPage
              podSearch={podSearch}
              setPodSearch={setPodSearch}
              podStatusFilter={podStatusFilter}
              setPodStatusFilter={setPodStatusFilter}
              availablePodStatuses={availablePodStatuses}
              podNameRegexError={podNameRegexError}
              podsAutoRefreshEnabled={podsAutoRefreshEnabled}
              setPodsAutoRefreshEnabled={setPodsAutoRefreshEnabled}
              podsAutoRefreshSeconds={podsAutoRefreshSeconds}
              setPodsAutoRefreshSeconds={setPodsAutoRefreshSeconds}
              sortedPods={sortedPods}
              selectedPodRefSet={selectedPodRefSet}
              selectedPodCount={selectedPods.length}
              togglePodRefSelection={togglePodRefSelection}
              setPodRefsSelection={setPodRefsSelection}
              selectedPodEvictPermission={selectedPodActionPermission('edit')}
              selectedPodDeletePermission={selectedPodActionPermission('delete')}
              toggleSort={toggleSort}
              sortMark={sortMark}
              selectedPod={selectedPod}
              selectPod={selectPod}
              isDegradedReady={isDegradedReady}
              showWarningPopover={showWarningPopover}
              scheduleWarningPopoverHide={scheduleWarningPopoverHide}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              safe={safe}
              openManifestTab={openManifestTab}
              openPodLogsTab={openPodLogsTab}
              allAllowed={allAllowed}
              openPodExecTab={openPodExecTab}
              evictPodByRef={evictPodByRef}
              deletePodByRef={deletePodByRef}
              setSelectedPod={setSelectedPod}
              refreshAll={refreshAll}
              deleteSelectedPods={deleteSelectedPods}
              evictSelectedPods={evictSelectedPods}
            />
          )}

          {activeNav === 'workloads' && (
            <WorkloadsPage
              workloadSearch={workloadSearch}
              setWorkloadSearch={setWorkloadSearch}
              sortedWorkloads={sortedWorkloads}
              toggleSort={toggleSort}
              sortMark={sortMark}
              isDegradedReady={isDegradedReady}
              showWarningPopover={showWarningPopover}
              scheduleWarningPopoverHide={scheduleWarningPopoverHide}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              allAllowed={allAllowed}
              safe={safe}
              openManifestTab={openManifestTab}
              openEditTab={openEditTab}
              openWorkloadLogsTab={openWorkloadLogsTab}
              openScaleModal={openScaleModal}
              setDeploymentName={setDeploymentName}
              setDeploymentNamespace={setDeploymentNamespace}
              restartDeployment={restartDeployment}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
            />
          )}

          {activeNav === 'nodes' && (
            <NodesPage
              availableNodeLabelKeys={availableNodeLabelKeys}
              applyNodeLabelKey={applyNodeLabelKey}
              availableNodeLabelValues={availableNodeLabelValues}
              parsedNodeLabelFilter={parsedNodeLabelFilter}
              applyNodeLabelValue={applyNodeLabelValue}
              nodeLabelFilter={nodeLabelFilter}
              setNodeLabelFilter={setNodeLabelFilter}
              sortedNodes={sortedNodes}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              allAllowed={allAllowed}
              selectedNamespaces={selectedNamespaces}
              permissionInfo={permissionInfo}
              primaryNamespace={primaryNamespace}
              safe={safe}
              openNodePodsTab={openNodePodsTab}
              openDrainModal={openDrainModal}
              uncordonNodeAction={uncordonNodeAction}
              openManifestTab={openManifestTab}
              openEditTab={openEditTab}
            />
          )}

          {activeNav === 'events' && <EventsPage sortedEvents={sortedEvents} />}

          {isInsightsView && (
            <InsightsPage
              categoryLabel={activeInsightCategoryLabel}
              showAllInsightChecks={showAllInsightChecks}
              setShowAllInsightChecks={setShowAllInsightChecks}
              showSuppressedInsights={showSuppressedInsights}
              setShowSuppressedInsights={setShowSuppressedInsights}
              refreshInsights={refreshInsights}
              insightsSummary={insightsSummary}
              selectAllInsightTypes={selectAllInsightTypes}
              clearInsightTypes={clearInsightTypes}
              availableInsightTypes={availableInsightTypes}
              selectedInsightTypes={selectedInsightTypes}
              toggleInsightType={toggleInsightType}
              sortedInsights={sortedInsights}
              groupedInsights={groupedInsights}
              openInsightResource={openInsightResource}
              openInsightLogs={openInsightLogs}
              safe={safe}
              setInsightSuppressed={setInsightSuppressed}
            />
          )}

          {activeNav === 'services' && (
            <ServicesPage
              serviceSearch={serviceSearch}
              setServiceSearch={setServiceSearch}
              sortedServices={sortedServices}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              safe={safe}
              openManifestTab={openManifestTab}
              allAllowed={allAllowed}
              openEditTab={openEditTab}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
            />
          )}

          {activeNav === 'clusterroles' && (
            <ClusterRolesPage
              sortedClusterRoles={sortedClusterRoles}
              sortedClusterRoleBindings={sortedClusterRoleBindings}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              primaryNamespace={primaryNamespace}
              safe={safe}
              openManifestTab={openManifestTab}
              allAllowed={allAllowed}
              openEditTab={openEditTab}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
            />
          )}

          {activeNav === 'rbacroles' && (
            <NamespacedRolesPage
              sortedRbacRoles={sortedRbacRoles}
              sortedRoleBindings={sortedRoleBindings}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              safe={safe}
              openManifestTab={openManifestTab}
              allAllowed={allAllowed}
              openEditTab={openEditTab}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
            />
          )}

          {activeNav === 'serviceaccounts' && (
            <ServiceAccountsPage
              serviceAccountSearch={serviceAccountSearch}
              setServiceAccountSearch={setServiceAccountSearch}
              sortedServiceAccounts={sortedServiceAccounts}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              safe={safe}
              openManifestTab={openManifestTab}
              allAllowed={allAllowed}
              openEditTab={openEditTab}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
            />
          )}

          {activeNav === 'ingresses' && (
            <IngressesPage
              ingressSearch={ingressSearch}
              setIngressSearch={setIngressSearch}
              sortedIngresses={sortedIngresses}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              safe={safe}
              openManifestTab={openManifestTab}
              allAllowed={allAllowed}
              openEditTab={openEditTab}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
            />
          )}

          {activeNav === 'configmaps' && (
            <ConfigMapsPage
              configMapSearch={configMapSearch}
              setConfigMapSearch={setConfigMapSearch}
              sortedConfigMaps={sortedConfigMaps}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              safe={safe}
              openManifestTab={openManifestTab}
              allAllowed={allAllowed}
              openEditTab={openEditTab}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
            />
          )}

          {activeNav === 'crds' && (
            <CRDsPage
              crdSearch={crdSearch}
              setCRDSearch={setCRDSearch}
              sortedCRDs={sortedCRDs}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              safe={safe}
              openManifestTab={openManifestTab}
              primaryNamespace={primaryNamespace}
              allAllowed={allAllowed}
              openEditTab={openEditTab}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
              selectedCRD={selectedCRD}
              selectedCRDGroup={selectedCRDGroup}
              customResourceSearch={customResourceSearch}
              setCustomResourceSearch={setCustomResourceSearch}
              sortedCustomResources={sortedCustomResources}
              customResourceDefinition={customResourceDefinition}
              selectCRDGroup={handleCRDGroupSelect}
            />
          )}

          {activeNav === 'secrets' && (
            <SecretsPage
              secretSearch={secretSearch}
              setSecretSearch={setSecretSearch}
              sortedSecrets={sortedSecrets}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              currentUser={currentUser}
              allAllowed={allAllowed}
              safe={safe}
              openManifestTab={openManifestTab}
              openEditTab={openEditTab}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
            />
          )}

          {activeNav === 'pvcs' && (
            <PVCsPage
              pvcSearch={pvcSearch}
              setPVCSearch={setPVCSearch}
              sortedPVCs={sortedPVCs}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              safe={safe}
              openManifestTab={openManifestTab}
              allAllowed={allAllowed}
              openEditTab={openEditTab}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
            />
          )}

          {activeNav === 'pvs' && (
            <PVsPage
              pvSearch={pvSearch}
              setPVSearch={setPVSearch}
              sortedPVs={sortedPVs}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              safe={safe}
              openManifestTab={openManifestTab}
              primaryNamespace={primaryNamespace}
              allAllowed={allAllowed}
              openEditTab={openEditTab}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
            />
          )}

          {activeNav === 'storageclasses' && (
            <StorageClassesPage
              sortedStorageClasses={sortedStorageClasses}
              toggleSort={toggleSort}
              sortMark={sortMark}
              makeAction={makeAction}
              permissionInfo={permissionInfo}
              safe={safe}
              openManifestTab={openManifestTab}
              primaryNamespace={primaryNamespace}
              allAllowed={allAllowed}
              openEditTab={openEditTab}
              deleteResourceByRef={deleteResourceByRef}
              refreshAll={refreshAll}
            />
          )}

          {activeNav === 'user-management' && (
            <UserManagementPage
              managedUsers={managedUsers}
              managedRoles={managedRoles}
              configImportInputRef={configImportInputRef}
              exportAdminConfig={exportAdminConfig}
              openConfigImportPicker={openConfigImportPicker}
              importAdminConfigFile={importAdminConfigFile}
              openCreateUserModal={openCreateUserModal}
              refreshUsers={refreshUsers}
              updateUserRole={updateUserRole}
              resetLocalUserPassword={resetLocalUserPassword}
              deleteUser={deleteUser}
              openCreateRoleModal={openCreateRoleModal}
              normalizeRolePermissions={normalizeRolePermissions}
              openEditRoleModal={openEditRoleModal}
              deleteRole={deleteRole}
              googleAuthConfigured={googleAuthConfigured}
              googleConfig={googleConfig}
              googleMappings={googleMappings}
              setShowGoogleConfigModal={setShowGoogleConfigModal}
              setShowGoogleMappingsModal={setShowGoogleMappingsModal}
              disableGoogleAuth={disableGoogleAuth}
              oidcAuthConfigured={oidcAuthConfigured}
              oidcConfig={oidcConfig}
              oidcMappings={oidcMappings}
              openOIDCConfigModal={openOIDCConfigModal}
              openEntraConfigModal={openEntraConfigModal}
              setShowOIDCMappingsModal={setShowOIDCMappingsModal}
              disableOIDCAuth={disableOIDCAuth}
              safe={safe}
            />
          )}

          {activeNav === 'cluster-health' && (
            <ClusterHealthPage
              nodes={nodes}
              pods={pods}
              workloads={workloads}
              services={services}
              ingresses={ingresses}
              pvcs={pvcs}
              pvs={pvs}
              storageClasses={storageClasses}
              events={events}
              runningPodsHealth={runningPodsHealth}
              formatMilliValue={formatMilliValue}
              formatByteValue={formatByteValue}
              formatGPURequestLabel={formatGPURequestLabel}
            />
          )}

          {activeNav === 'apply' && (
            <ApplyYamlPage
              selectedTemplate={selectedTemplate}
              loadTemplate={loadTemplate}
              applyTemplates={APPLY_TEMPLATES}
              yamlText={yamlText}
              setYamlText={setYamlText}
              safe={safe}
              applyYaml={applyYaml}
              primaryNamespace={primaryNamespace}
              permissionInfo={permissionInfo}
            />
          )}
            </>
          )}
          </div>
        </div>

        <BottomDock
          showBottomDock={showBottomDock}
          startDockResize={startDockResize}
          bottomTabs={bottomTabs}
          activeBottomTabId={activeBottomTabId}
          setActiveBottomTabId={setActiveBottomTabId}
          closeTab={closeTab}
          activeBottomTab={activeBottomTab}
          upsertTab={upsertTab}
          scheduleLogsScrollToBottom={scheduleLogsScrollToBottom}
          refreshLogTab={refreshLogTab}
          changeLogContainer={changeLogContainer}
          refreshManifestTab={refreshManifestTab}
          revealSecretManifestTab={revealSecretManifestTab}
          handleLogsScroll={handleLogsScroll}
          logsOutputRef={logsOutputRef}
          logsEndRef={logsEndRef}
          applyEditTab={applyEditTab}
          permissionInfo={permissionInfo}
          primaryNamespace={primaryNamespace}
          openNodePodsTab={openNodePodsTab}
          makeAction={makeAction}
          safe={safe}
          openManifestTab={openManifestTab}
          openPodLogsTab={openPodLogsTab}
          evictPodByRef={evictPodByRef}
          setStatus={setStatus}
          refreshAll={refreshAll}
          isDegradedReady={isDegradedReady}
          openPodExecTab={openPodExecTab}
          deletePodByRef={deletePodByRef}
          selectedPod={selectedPod}
          setSelectedPod={setSelectedPod}
          execTerminalHostRef={execTerminalHostRef}
        />
        </div>

        <WarningPopover
          warningPopover={warningPopover}
          cancelHide={cancelWarningPopoverHide}
          scheduleHide={scheduleWarningPopoverHide}
        />
      </section>

      <GoogleConfigModal
        open={showGoogleConfigModal}
        config={googleConfig}
        onClose={() => setShowGoogleConfigModal(false)}
        onChange={(field, value) => setGoogleConfig((prev) => ({ ...prev, [field]: value }))}
        onTest={() => safeBottomNotice(testGoogleConfig)}
        onSave={() => safe(async () => { await saveGoogleConfig(); setShowGoogleConfigModal(false); })}
      />

      <GoogleMappingsModal
        open={showGoogleMappingsModal}
        onClose={() => setShowGoogleMappingsModal(false)}
        mappings={googleMappings}
        groupEmail={newGoogleGroupEmail}
        role={newGoogleRole}
        editingGroupEmail={editingGoogleGroupEmail}
        roles={managedRoles}
        onGroupEmailChange={setNewGoogleGroupEmail}
        onRoleChange={setNewGoogleRole}
        onSave={() => safe(saveGoogleMapping)}
        onCancel={resetGoogleMappingForm}
        onEdit={editGoogleMapping}
        onDelete={(groupEmail) => safe(() => deleteGoogleMapping(groupEmail))}
      />

      <OIDCConfigModal
        open={showOIDCConfigModal}
        config={oidcConfigDraft}
        mode={oidcConfigModalMode}
        onClose={() => setShowOIDCConfigModal(false)}
        onChange={(field, value) => setOIDCConfigDraft((prev) => ({ ...prev, [field]: value }))}
        onTest={() => safeBottomNotice(testOIDCConfig)}
        onSave={() => safe(async () => { await saveOIDCConfig(); setShowOIDCConfigModal(false); })}
      />

      <OIDCMappingsModal
        open={showOIDCMappingsModal}
        onClose={() => setShowOIDCMappingsModal(false)}
        providerName={oidcConfig.provider_name || 'OpenID Connect'}
        mappings={oidcMappings}
        groupName={newOIDCGroupName}
        role={newOIDCRole}
        editingGroupName={editingOIDCGroupName}
        roles={managedRoles}
        onGroupNameChange={setNewOIDCGroupName}
        onRoleChange={setNewOIDCRole}
        onSave={() => safe(saveOIDCMapping)}
        onCancel={resetOIDCMappingForm}
        onEdit={editOIDCMapping}
        onDelete={(groupName) => safe(() => deleteOIDCMapping(groupName))}
      />

      <RoleModal
        open={showRoleModal}
        onClose={() => setShowRoleModal(false)}
        editingRoleName={editingRoleName}
        roleFormName={roleFormName}
        roleFormMode={roleFormMode}
        roleFormNamespaces={roleFormNamespaces}
        roleFormPermissions={roleFormPermissions}
        namespaces={namespaces}
        roleResources={ROLE_RESOURCES}
        rolesOptionsFor={roleOptionsFor}
        resolveRoleLevel={resolveRoleLevel}
        permissionLevel={permissionLevel}
        setRoleFormName={setRoleFormName}
        setRoleFormMode={setRoleFormMode}
        onSelectAllNamespaces={() => setRoleFormNamespaces(namespaces)}
        onClearNamespaces={() => setRoleFormNamespaces([])}
        toggleRoleNamespace={toggleRoleNamespace}
        setRolePermissionLevel={setRolePermissionLevel}
        setAllRolePermissionLevels={setAllRolePermissionLevels}
        onSave={() => safe(saveRole)}
      />

      <CreateUserModal
        open={showCreateUserModal}
        onClose={() => {
          setShowCreateUserModal(false);
          setNewUsername('');
          setNewUserPassword('');
          setNewUserRole(managedRoles[0]?.name || 'viewer');
        }}
        username={newUsername}
        setUsername={setNewUsername}
        password={newUserPassword}
        setPassword={setNewUserPassword}
        role={newUserRole}
        setRole={setNewUserRole}
        roles={managedRoles}
        onSubmit={() => safe(createUser)}
      />

      <ProfileModal
        open={showProfile}
        onClose={() => setShowProfile(false)}
        currentUser={currentUser}
        selectedNamespaces={selectedNamespaces}
        themeOptions={THEME_OPTIONS}
        themePreference={themePreference}
        resolvedTheme={resolvedTheme}
        onThemeChange={setThemePreference}
        onLogout={() => { void logout(); }}
      />

      <DrainModal
        open={showDrainModal}
        onClose={() => setShowDrainModal(false)}
        drainTargetNode={drainTargetNode}
        drainForce={drainForce}
        onForceChange={setDrainForce}
        onDrain={() => safe(() => drainNodeAction(drainTargetNode))}
      />

      <ScaleModal
        open={showScaleModal}
        onClose={() => setShowScaleModal(false)}
        scaleTargetKind={scaleTargetKind}
        deploymentNamespace={deploymentNamespace}
        deploymentName={deploymentName}
        replicas={replicas}
        onReplicasChange={setReplicas}
        canApply={allAllowed(
          { allowed: Boolean((deploymentNamespace || primaryNamespace).trim()), reason: 'Select namespace first' },
          permissionInfo('workloads', 'edit', deploymentNamespace || primaryNamespace)
        ).allowed}
        applyReason={allAllowed(
          { allowed: Boolean((deploymentNamespace || primaryNamespace).trim()), reason: 'Select namespace first' },
          permissionInfo('workloads', 'edit', deploymentNamespace || primaryNamespace)
        ).reason}
        onApply={() => safe(scaleWorkload)}
      />

      <PasswordPromptModal
        open={showPasswordPromptModal}
        title="Reset User Password"
        subjectLabel="User"
        subjectValue={passwordPromptUsername}
        password={passwordPromptValue}
        setPassword={setPasswordPromptValue}
        confirmPassword={passwordPromptConfirm}
        setConfirmPassword={setPasswordPromptConfirm}
        onClose={() => {
          setShowPasswordPromptModal(false);
          setPasswordPromptUsername('');
          setPasswordPromptValue('');
          setPasswordPromptConfirm('');
        }}
        onSubmit={() => safe(applyPasswordReset)}
      />

      {bottomNotice ? (
        <div className={`bottom-notice bottom-notice-${bottomNotice.type}`}>
          {bottomNotice.text}
        </div>
      ) : null}
    </div>
  );
}
