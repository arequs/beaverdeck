import { useEffect, useMemo, useState } from 'react';
import { AUTH_EXPIRED_EVENT, createApi, publicApi } from '../lib/api.js';
import { NAMESPACE_STORAGE_KEY } from '../lib/appConstants.js';
import { defaultRolePermissions, getStoredThemePreference, normalizeRolePermissions } from '../lib/appUtils.js';
import { appBasePath, withBasePath } from '../lib/paths.js';
import { currentDestination, loginDestination, readOAuthDestination, saveOAuthDestination } from '../lib/returnTo.js';
import { useNavigate } from 'react-router';
import { AUTH_API_ROUTES, LOGIN_ROUTE } from '../shared/routes/index.js';

const DEFAULT_AUTH_PROVIDERS = {
  appVersion: '',
  local: true,
  google: { enabled: false, hosted_domain: '' },
  oidc: { enabled: false, provider_name: 'OpenID Connect', hosted_domain: '' },
  entra: { enabled: false, provider_name: 'Azure Entra ID', hosted_domain: '' }
};

const DEFAULT_BOOTSTRAP_STATE = {
  initialized: true
};

const DEFAULT_CURRENT_USER = {
  username: '',
  role: 'viewer',
  roleMode: 'viewer',
  authSource: 'local',
  clusterName: '',
  appVersion: '',
  latestVersion: '',
  permissions: defaultRolePermissions()
};

const LEGACY_AUTH_STORAGE_KEY = 'beaverdeck-auth';

function clearStoredAuth() {
  localStorage.removeItem(LEGACY_AUTH_STORAGE_KEY);
}

function persistNamespaces(nextNamespaces) {
  localStorage.setItem(NAMESPACE_STORAGE_KEY, JSON.stringify(nextNamespaces));
}

function clearStoredNamespaces() {
  localStorage.removeItem(NAMESPACE_STORAGE_KEY);
}

function getStoredNamespaces() {
  try {
    const raw = localStorage.getItem(NAMESPACE_STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((item) => typeof item === 'string') : [];
  } catch {
    return [];
  }
}

function consumeAuthCallback() {
  if (typeof window === 'undefined') return null;
  const rawHash = window.location.hash.replace(/^#/, '');
  if (!rawHash) return null;
  const params = new URLSearchParams(rawHash);
  if (!params.has('auth_session') && !params.has('auth_error')) {
    return null;
  }
  const result = {
    username: params.get('auth_user') || '',
    token: params.get('auth_session') || '',
    error: params.get('auth_error') || ''
  };
  const cleanURL = `${window.location.pathname}${window.location.search}`;
  window.history.replaceState(null, '', cleanURL);
  return result;
}

export default function useAuthSession({
  selectedNamespaces,
  setNamespaces,
  setSelectedNamespaces,
  setThemePreference,
  beforeLogout,
  afterLogout
}) {
  const navigate = useNavigate();

  function navigateToDestination(destination) {
    const base = appBasePath();
    const relative = destination ? destination.slice(base.length) : '';
    navigate(!relative || relative === '/' ? '/dashboard' : relative, { replace: true });
  }

  function navigateToLogin(destination) {
    const query = destination ? `?${new URLSearchParams({ returnTo: destination })}` : '';
    navigate(`${LOGIN_ROUTE}${query}`, { replace: true });
  }

  const [usernameInput, setUsernameInput] = useState('');
  const [passwordInput, setPasswordInput] = useState('');
  const [token, setToken] = useState('');
  const [username, setUsername] = useState('');
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [authBootstrapping, setAuthBootstrapping] = useState(true);
  const [authProviders, setAuthProviders] = useState(DEFAULT_AUTH_PROVIDERS);
  const [authError, setAuthError] = useState('');
  const [currentUser, setCurrentUser] = useState(DEFAULT_CURRENT_USER);
  const [bootstrapState, setBootstrapState] = useState(DEFAULT_BOOTSTRAP_STATE);

  const api = useMemo(() => (token && username ? createApi(token, username) : null), [token, username]);

  async function loadAuthProviders() {
    try {
      const data = await publicApi(AUTH_API_ROUTES.providers);
      setAuthProviders({
        appVersion: data.appVersion || '',
        local: data.local !== false,
        google: {
          enabled: Boolean(data.google?.enabled),
          hosted_domain: data.google?.hosted_domain || ''
        },
        oidc: {
          enabled: Boolean(data.oidc?.enabled),
          provider_name: data.oidc?.provider_name || 'OpenID Connect',
          hosted_domain: data.oidc?.hosted_domain || ''
        },
        entra: {
          enabled: Boolean(data.entra?.enabled),
          provider_name: data.entra?.provider_name || 'Azure Entra ID',
          hosted_domain: data.entra?.hosted_domain || ''
        }
      });
    } catch {
      setAuthProviders(DEFAULT_AUTH_PROVIDERS);
    }
  }

  async function loadBootstrapStatus() {
    try {
      const data = await publicApi(AUTH_API_ROUTES.bootstrapStatus);
      setBootstrapState({ initialized: Boolean(data.initialized) });
      return Boolean(data.initialized);
    } catch {
      setBootstrapState(DEFAULT_BOOTSTRAP_STATE);
      return true;
    }
  }

  async function initializeSession(nextUsername, nextToken, options = {}) {
    const { restoreInputs = false } = options;
    const probeApi = createApi(nextToken, nextUsername);
    const me = await probeApi('/api/me');
    const nsData = await probeApi('/api/namespaces');
    const nsItems = nsData.items || [];
    const savedNamespaces = getStoredNamespaces();
    const restoredNamespaces = savedNamespaces.filter((ns) => nsItems.includes(ns));
    const defaultNamespaces = nsItems.includes('default')
      ? ['default']
      : (nsItems.length > 0 ? [nsItems[0]] : []);
    const effectiveNamespaces = restoredNamespaces.length > 0
      ? restoredNamespaces
      : defaultNamespaces;

    setToken(nextToken);
    setUsername(nextUsername);
    setIsLoggedIn(true);
    setCurrentUser({
      username: me.username || '',
      role: me.role || 'viewer',
      roleMode: me.roleMode || 'viewer',
      authSource: me.authSource || 'local',
      clusterName: me.clusterName || '',
      appVersion: me.appVersion || '',
      latestVersion: me.latestVersion || '',
      permissions: normalizeRolePermissions(me.permissions)
    });
    setThemePreference(getStoredThemePreference(nextUsername));
    setNamespaces(nsItems);
    setSelectedNamespaces(effectiveNamespaces);
    if (restoreInputs) {
      setUsernameInput(nextUsername);
      setPasswordInput('');
    }
    persistNamespaces(effectiveNamespaces);
  }

  async function login() {
    setAuthError('');
    try {
      if (!usernameInput.trim() || !passwordInput.trim()) {
        throw new Error('username and password are required');
      }
      const data = await publicApi(AUTH_API_ROUTES.login, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: usernameInput.trim(),
          password: passwordInput.trim()
        })
      });
      await initializeSession(data.username, data.token);
      setPasswordInput('');
      navigateToDestination(loginDestination());
    } catch (err) {
      setAuthError(err.message || String(err));
    }
  }

  async function completeBootstrap(adminUsername, password) {
    await publicApi(AUTH_API_ROUTES.bootstrapComplete, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: adminUsername.trim(),
        password: password.trim()
      })
    });
    setBootstrapState({ initialized: true });
    await loadAuthProviders();
    setUsernameInput(adminUsername.trim());
    setPasswordInput('');
    setAuthError('Initialization complete. Sign in as the admin user.');
  }

  function startGoogleLogin() {
    saveOAuthDestination(loginDestination());
    window.location.href = withBasePath(AUTH_API_ROUTES.googleStart);
  }

  function startOIDCLogin() {
    saveOAuthDestination(loginDestination());
    window.location.href = withBasePath(AUTH_API_ROUTES.oidcStart);
  }

  function startEntraLogin() {
    saveOAuthDestination(loginDestination());
    window.location.href = withBasePath(AUTH_API_ROUTES.entraStart);
  }

  async function logout(options = {}) {
    const { revokeRemote = true, reason = '' } = options;
    const activeToken = token;
    const destination = currentDestination();

    if (beforeLogout) {
      await beforeLogout();
    }

    if (revokeRemote && activeToken) {
      try {
        await publicApi(AUTH_API_ROUTES.logout, {
          method: 'POST',
          headers: { Authorization: `Bearer ${activeToken}` }
        });
      } catch {
        // ignore logout failures
      }
    }

    navigateToLogin(destination);
    setIsLoggedIn(false);
    setToken('');
    setUsername('');
    setUsernameInput('');
    setPasswordInput('');
    clearStoredAuth();
    clearStoredNamespaces();
    setAuthError(reason);
    setCurrentUser(DEFAULT_CURRENT_USER);
    setAuthProviders((prev) => prev || DEFAULT_AUTH_PROVIDERS);
    setThemePreference('auto');
    setNamespaces([]);
    setSelectedNamespaces([]);

    if (afterLogout) {
      afterLogout();
    }
  }

  useEffect(() => {
    if (!isLoggedIn) return;
    persistNamespaces(selectedNamespaces);
  }, [isLoggedIn, selectedNamespaces]);

  useEffect(() => {
    let cancelled = false;
    async function restoreSession() {
      try {
        await loadAuthProviders();
        const initialized = await loadBootstrapStatus();
        if (!initialized) {
          if (window.location.pathname !== withBasePath(LOGIN_ROUTE)) navigateToLogin(currentDestination());
          return;
        }
        const authCallback = consumeAuthCallback();
        if (authCallback?.error) {
          if (!cancelled) {
            setAuthError(authCallback.error);
          }
          navigateToLogin(readOAuthDestination() || loginDestination());
          clearStoredAuth();
          clearStoredNamespaces();
          return;
        }
        if (authCallback?.username && authCallback?.token) {
          await initializeSession(authCallback.username, authCallback.token);
          navigateToDestination(readOAuthDestination(true));
          return;
        }
        const destination = currentDestination();
        if (window.location.pathname !== withBasePath(LOGIN_ROUTE)) navigateToLogin(destination);
        clearStoredAuth();
      } catch {
        navigateToLogin(readOAuthDestination() || currentDestination());
        clearStoredAuth();
        clearStoredNamespaces();
        if (!cancelled) {
          setToken('');
          setUsername('');
          setIsLoggedIn(false);
          setCurrentUser(DEFAULT_CURRENT_USER);
          setNamespaces([]);
          setSelectedNamespaces([]);
        }
      } finally {
        if (!cancelled) {
          setAuthBootstrapping(false);
        }
      }
    }
    void restoreSession();
    return () => {
      cancelled = true;
    };
  }, [setNamespaces, setSelectedNamespaces, setThemePreference]);

  useEffect(() => {
    const handlePageHide = () => {
      if (!token) {
        return;
      }

      fetch(withBasePath(AUTH_API_ROUTES.logout), {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
        // Indicates that the browser will keep the associated request alive if the page is unloaded before the request is complete
        keepalive: true
      }).catch(() => {
        // Without it, a failed request logs an unhandled promise rejection.
      });
    };

    window.addEventListener('pagehide', handlePageHide);

    return () => window.removeEventListener('pagehide', handlePageHide);
  }, [token]);

  useEffect(() => {
    const handleAuthExpired = (event) => {
      const message = event?.detail?.message || 'Session expired. Please sign in again.';
      void logout({ revokeRemote: false, reason: message });
    };
    window.addEventListener(AUTH_EXPIRED_EVENT, handleAuthExpired);
    return () => window.removeEventListener(AUTH_EXPIRED_EVENT, handleAuthExpired);
  }, [token]);

  return {
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
    reloadAuthProviders: loadAuthProviders,
    startGoogleLogin,
    startOIDCLogin,
    startEntraLogin
  };
}
