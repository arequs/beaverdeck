import React, { useEffect, useRef } from 'react';
import { Boxes, Check, ChevronDown, ChevronRight, RefreshCw, UserRound } from 'lucide-react';
import { withBasePath } from '../lib/paths.js';
import { GoogleIcon, OAuthIcon } from './AuthIcons.jsx';
import PasswordField from './PasswordField.jsx';

export function LoginScreen({
  title,
  message,
  usernameInput,
  setUsernameInput,
  passwordInput,
  setPasswordInput,
  login,
  authProviders,
  startGoogleLogin,
  startOIDCLogin,
  authError,
  showInputs = true,
  appVersion = ''
}) {
  return (
    <div className="login-screen">
      <div className="login-layout">
        <div className="login-card">
          <div className="login-brand-lockup">
            <img className="login-logo" src={withBasePath('/logo.png')} alt="" aria-hidden="true" />
            <div className="login-brand-name">BeaverDeck</div>
          </div>
          {title ? <h1>{title}</h1> : null}
          {message ? <p>{message}</p> : null}
          {showInputs ? (
            <>
              <input
                type="text"
                value={usernameInput}
                onChange={(e) => setUsernameInput(e.target.value)}
                placeholder="Username"
                onKeyDown={(e) => {
                  if (e.key === 'Enter') login();
                }}
              />
              <PasswordField
                value={passwordInput}
                onChange={(e) => setPasswordInput(e.target.value)}
                placeholder="Password"
                onKeyDown={(e) => {
                  if (e.key === 'Enter') login();
                }}
              />
              <button onClick={login}>Login</button>
              {authProviders.google?.enabled ? (
                <button className="secondary google-login-button" onClick={startGoogleLogin}>
                  <GoogleIcon />
                  Sign in with Google{authProviders.google?.hosted_domain ? ` (${authProviders.google.hosted_domain})` : ''}
                </button>
              ) : null}
              {authProviders.oidc?.enabled ? (
                <button className="secondary google-login-button" onClick={startOIDCLogin}>
                  <span className="oauth-icon-wrap"><OAuthIcon /></span>
                  Sign in with {authProviders.oidc?.provider_name || 'OpenID Connect'}{authProviders.oidc?.hosted_domain ? ` (${authProviders.oidc.hosted_domain})` : ''}
                </button>
              ) : null}
              {authError ? <div className="error-text">{authError}</div> : null}
            </>
          ) : null}
        </div>
        {appVersion ? <div className="small-hint login-version">Version {appVersion}</div> : null}
      </div>
    </div>
  );
}

export function BootstrapSetupScreen({
  adminUsername,
  setAdminUsername,
  adminPassword,
  setAdminPassword,
  adminPasswordConfirm,
  setAdminPasswordConfirm,
  onComplete,
  statusText,
  errorText
}) {
  return (
    <div className="login-screen">
      <div className="login-card bootstrap-card">
        <div className="login-brand-lockup">
          <img className="login-logo" src={withBasePath('/logo.png')} alt="" aria-hidden="true" />
          <div className="login-brand-name">BeaverDeck</div>
        </div>
        <h1>BeaverDeck Initialization</h1>
        <p>Set the initial admin username and password.</p>
        <input
          type="text"
          value={adminUsername}
          onChange={(e) => setAdminUsername(e.target.value)}
          placeholder="Admin username"
          onKeyDown={(e) => {
            if (e.key === 'Enter') onComplete();
          }}
        />
        <PasswordField
          value={adminPassword}
          onChange={(e) => setAdminPassword(e.target.value)}
          placeholder="Admin password"
          onKeyDown={(e) => {
            if (e.key === 'Enter') onComplete();
          }}
        />
        <PasswordField
          value={adminPasswordConfirm}
          onChange={(e) => setAdminPasswordConfirm(e.target.value)}
          placeholder="Confirm admin password"
          onKeyDown={(e) => {
            if (e.key === 'Enter') onComplete();
          }}
        />
        <button onClick={onComplete}>Initialize</button>
        {statusText ? <div className="small-hint">{statusText}</div> : null}
        {errorText ? <div className="error-text">{errorText}</div> : null}
      </div>
    </div>
  );
}

export function SidebarNav({
  activeNav,
  clusterName,
  selectedNamespaces,
  namespaces,
  setSelectedNamespaces,
  toggleNamespace,
  visibleMenu,
  expandedNavSections,
  toggleNavSection,
  handleNavChange
}) {
  const allNamespacesRef = useRef(null);
  const allNamespacesSelected = namespaces.length > 0 && selectedNamespaces.length === namespaces.length;
  const someNamespacesSelected = selectedNamespaces.length > 0 && !allNamespacesSelected;

  useEffect(() => {
    if (allNamespacesRef.current) {
      allNamespacesRef.current.indeterminate = someNamespacesSelected;
    }
  }, [someNamespacesSelected]);

  return (
    <aside className="sidebar">
      <div className="brand">
        <img className="brand-logo brand-logo-large" src={withBasePath('/logo.png')} alt="" aria-hidden="true" />
        <div className="brand-copy">
          <div className="brand-title">BeaverDeck</div>
          <div className="brand-cluster">{clusterName || 'Current cluster'}</div>
        </div>
      </div>

      <div className="cluster-card">
        <div className="cluster-card-head">
          <div className="small-label">Namespaces</div>
        </div>
        <div className="ns-picker-popover ns-picker-inline">
          <label className="namespace-select-all">
            <input
              ref={allNamespacesRef}
              type="checkbox"
              checked={allNamespacesSelected}
              onChange={(event) => setSelectedNamespaces(event.target.checked ? namespaces : [])}
              aria-label={allNamespacesSelected ? 'Unselect all namespaces' : 'Select all namespaces'}
            />
            <span>Select all</span>
          </label>
          <div className="ns-picker-list">
            {namespaces.map((ns) => (
              <label key={ns} className="ns-picker-item">
                <input type="checkbox" checked={selectedNamespaces.includes(ns)} onChange={() => toggleNamespace(ns)} />
                <span>{ns}</span>
              </label>
            ))}
          </div>
        </div>
      </div>

      <div className="nav-section-title">Navigation</div>
      {visibleMenu.map((group) => {
        const expanded = expandedNavSections?.has(group.section);
        return (
          <div key={group.section} className={`menu-group ${expanded ? 'expanded' : 'collapsed'}`}>
            <button
              type="button"
              className="menu-group-title"
              onClick={() => toggleNavSection(group.section)}
              aria-expanded={expanded}
            >
              {expanded ? <ChevronDown size={13} strokeWidth={2} aria-hidden="true" /> : <ChevronRight size={13} strokeWidth={2} aria-hidden="true" />}
              <span>{group.section}</span>
            </button>
            {expanded && (
              <div className="menu-group-items">
                {group.items.map((item) => (
                  <button
                    key={item.id}
                    className={`nav-item ${activeNav === item.id ? 'active' : ''}`}
                    onClick={() => handleNavChange(item.id)}
                  >
                    {item.label}
                  </button>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </aside>
  );
}

export function WorkspaceHeader({ title, status, onRefresh, onProfile }) {
  return (
    <header className="topbar">
      <div className="topbar-title">
        <Boxes size={18} strokeWidth={1.8} aria-hidden="true" />
        <strong>{title}</strong>
      </div>
      <div className="top-actions">
        {status ? <span className="status-text">{status}</span> : <span className="status-text status-idle"><Check size={13} strokeWidth={2} aria-hidden="true" />Idle</span>}
        <button className="icon-text-button" onClick={onRefresh}>
          <RefreshCw size={15} strokeWidth={1.8} aria-hidden="true" />
          <span>Refresh</span>
        </button>
        <button className="icon-text-button" onClick={onProfile}>
          <UserRound size={15} strokeWidth={1.8} aria-hidden="true" />
          <span>Profile</span>
        </button>
      </div>
    </header>
  );
}
