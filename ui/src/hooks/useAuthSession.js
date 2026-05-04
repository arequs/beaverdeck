import { useEffect, useMemo, useState } from 'react';
import { AUTH_EXPIRED_EVENT, createApi, publicApi } from '../lib/api.js';
import { AUTH_STORAGE_KEY, NAMESPACE_STORAGE_KEY } from '../lib/appConstants.js';
import { defaultRolePermissions, getStoredThemePreference, normalizeRolePermissions } from '../lib/appUtils.js';
import { withBasePath } from '../lib/paths.js';

const DEFAULT_AUTH_PROVIDERS = {
  appVersion: '',
  local: true,
  google: { enabled: false, hosted_domain: '' },
  oidc: { enabled: false, provider_name: 'Custom OAuth', hosted_domain: '' }
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

function persistAuth(nextUsername, nextToken) {
  localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify({ username: nextUsername, token: nextToken }));
}

function clearStoredAuth() {
  localStorage.removeItem(AUTH_STORAGE_KEY);
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
      const data = await publicApi('/api/auth/providers');
      setAuthProviders({
        appVersion: data.appVersion || '',
        local: data.local !== false,
        google: {
          enabled: Boolean(data.google?.enabled),
          hosted_domain: data.google?.hosted_domain || ''
        },
        oidc: {
          enabled: Boolean(data.oidc?.enabled),
          provider_name: data.oidc?.provider_name || 'Custom OAuth',
          hosted_domain: data.oidc?.hosted_domain || ''
        }
      });
    } catch {
      setAuthProviders(DEFAULT_AUTH_PROVIDERS);
    }
  }

  async function loadBootstrapStatus() {
    try {
      const data = await publicApi('/api/auth/bootstrap/status');
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
    persistAuth(nextUsername, nextToken);
    persistNamespaces(effectiveNamespaces);
  }

  async function login() {
    setAuthError('');
    try {
      if (!usernameInput.trim() || !passwordInput.trim()) {
        throw new Error('username and password are required');
      }
      const data = await publicApi('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: usernameInput.trim(),
          password: passwordInput.trim()
        })
      });
      await initializeSession(data.username, data.token);
      setPasswordInput('');
    } catch (err) {
      setAuthError(err.message || String(err));
    }
  }

  async function completeBootstrap(tokenInput, password) {
    await publicApi('/api/auth/bootstrap/complete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        token: tokenInput.trim(),
        password: password.trim()
      })
    });
    setBootstrapState({ initialized: true });
    await loadAuthProviders();
    setUsernameInput('admin');
    setPasswordInput('');
    setAuthError('Initialization complete. Sign in as admin.');
  }

  function startGoogleLogin() {
    window.location.href = withBasePath('/api/auth/google/start');
  }

  function startOIDCLogin() {
    window.location.href = withBasePath('/api/auth/oidc/start');
  }

  async function logout(options = {}) {
    const { revokeRemote = true, reason = '' } = options;
    const activeToken = token;

    if (beforeLogout) {
      await beforeLogout();
    }

    if (revokeRemote && activeToken) {
      try {
        await publicApi('/api/auth/logout', {
          method: 'POST',
          headers: { Authorization: `Bearer ${activeToken}` }
        });
      } catch {
        // ignore logout failures
      }
    }

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
          return;
        }
        const authCallback = consumeAuthCallback();
        if (authCallback?.error) {
          if (!cancelled) {
            setAuthError(authCallback.error);
          }
          clearStoredAuth();
          clearStoredNamespaces();
          return;
        }
        if (authCallback?.username && authCallback?.token) {
          await initializeSession(authCallback.username, authCallback.token);
          return;
        }
        const raw = localStorage.getItem(AUTH_STORAGE_KEY);
        if (!raw) return;
        const parsed = JSON.parse(raw);
        if (!parsed || typeof parsed.username !== 'string' || typeof parsed.token !== 'string' || !parsed.username || !parsed.token) {
          clearStoredAuth();
          return;
        }
        await initializeSession(parsed.username, parsed.token, { restoreInputs: true });
      } catch {
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
    startOIDCLogin
  };
}
