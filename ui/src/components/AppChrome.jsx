import React from 'react';
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
                  Sign in with {authProviders.oidc?.provider_name || 'Custom OAuth'}{authProviders.oidc?.hosted_domain ? ` (${authProviders.oidc.hosted_domain})` : ''}
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
  bootstrapTokenInput,
  setBootstrapTokenInput,
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
        <p>Enter the bootstrap token from the application log, then set the admin password.</p>
        <input
          type="text"
          value={bootstrapTokenInput}
          onChange={(e) => setBootstrapTokenInput(e.target.value)}
          placeholder="Bootstrap token"
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
  setNsPickerOpen,
  nsPickerOpen,
  clusterName,
  selectedNamespaces,
  namespaces,
  setSelectedNamespaces,
  toggleNamespace,
  visibleMenu,
  handleNavChange
}) {
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
        <div className="small-label">Namespaces</div>
        <button className="ns-picker-btn" onClick={() => setNsPickerOpen((v) => !v)}>
          {selectedNamespaces.length ? `${selectedNamespaces.length} selected` : 'Select namespaces'}
        </button>
        {nsPickerOpen && (
          <div className="ns-picker-popover">
            <div className="ns-picker-actions">
              <button onClick={() => setSelectedNamespaces(namespaces)}>All</button>
              <button onClick={() => setSelectedNamespaces([])}>None</button>
            </div>
            <div className="ns-picker-list">
              {namespaces.map((ns) => (
                <label key={ns} className="ns-picker-item">
                  <input type="checkbox" checked={selectedNamespaces.includes(ns)} onChange={() => toggleNamespace(ns)} />
                  <span>{ns}</span>
                </label>
              ))}
            </div>
          </div>
        )}
        <div className="small-hint">{selectedNamespaces.join(', ') || 'Nothing selected'}</div>
      </div>

      <div className="nav-section-title">Menu</div>
      {visibleMenu.map((group) => (
        <div key={group.section} className="menu-group">
          <div className="menu-group-title">{group.section}</div>
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
      ))}
    </aside>
  );
}

export function WorkspaceHeader({ title, status, onRefresh, onProfile }) {
  return (
    <header className="topbar">
      <strong>{title}</strong>
      <div className="top-actions">
        {status ? <span className="status-text">{status}</span> : null}
        <button onClick={onRefresh}>Refresh</button>
        <button onClick={onProfile}>Profile</button>
      </div>
    </header>
  );
}
