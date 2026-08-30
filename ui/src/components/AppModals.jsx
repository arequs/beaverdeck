import React from 'react';
import PasswordField from './PasswordField.jsx';

export function GoogleConfigModal({ open, config, onClose, onChange, onTest, onSave }) {
  if (!open) return null;
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card google-auth-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Configure Google Auth</h3>
          <button className="modal-close" type="button" aria-label="Close" onClick={onClose}>×</button>
        </div>
        <p className="modal-muted">Configure OAuth login plus Google Admin Directory access for group-to-role mapping.</p>
        <div className="settings-form">
          <label className="settings-form-row">
            <span className="settings-form-label">Google Client ID</span>
            <input value={config.client_id} onChange={(e) => onChange('client_id', e.target.value)} placeholder="google client id" />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Google Client Secret</span>
            <PasswordField value={config.client_secret} onChange={(e) => onChange('client_secret', e.target.value)} placeholder="google client secret" />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Hosted Domain</span>
            <input value={config.hosted_domain} onChange={(e) => onChange('hosted_domain', e.target.value)} placeholder="hosted domain (optional)" />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Delegated Admin Email</span>
            <input value={config.delegated_admin_email} onChange={(e) => onChange('delegated_admin_email', e.target.value)} placeholder="delegated admin email" />
          </label>
          <label className="settings-form-row settings-form-row-wide">
            <span className="settings-form-label">Service Account JSON</span>
            <textarea
              rows={10}
              className="settings-form-textarea"
              value={config.service_account_json}
              onChange={(e) => onChange('service_account_json', e.target.value)}
              placeholder={'service account json\n{\n  "type": "service_account",\n  "client_email": "...",\n  "private_key": "-----BEGIN PRIVATE KEY-----\\n..."\n}'}
            />
          </label>
        </div>
        <div className="modal-actions">
          <button className="secondary" onClick={onTest}>Test Google auth</button>
          <button onClick={onSave}>Save</button>
        </div>
      </div>
    </div>
  );
}

export function GoogleMappingsModal({
  open,
  onClose,
  mappings,
  groupEmail,
  role,
  editingGroupEmail,
  roles,
  onGroupEmailChange,
  onRoleChange,
  onSave,
  onCancel,
  onEdit,
  onDelete
}) {
  if (!open) return null;
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card google-auth-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Configure Google Group Mapping</h3>
          <button className="modal-close" type="button" aria-label="Close" onClick={onClose}>×</button>
        </div>
        <p className="modal-muted">Map Google Workspace groups to BeaverDeck roles.</p>
        <div className="toolbar fixed-toolbar mapping-form">
          <input value={groupEmail} onChange={(e) => onGroupEmailChange(e.target.value)} placeholder="google-group@example.com" disabled={Boolean(editingGroupEmail)} />
          <select value={role} onChange={(e) => onRoleChange(e.target.value)}>
            {roles.map((item) => (
              <option key={item.name} value={item.name}>{item.name}</option>
            ))}
          </select>
          <button onClick={onSave}>{editingGroupEmail ? 'Update' : 'Add'}</button>
          {editingGroupEmail ? <button className="secondary" onClick={onCancel}>Cancel</button> : null}
        </div>
        {editingGroupEmail ? <div className="small-hint">Editing mapping for {editingGroupEmail}</div> : null}
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Google Group</th>
                <th>Role</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {mappings.map((item) => (
                <tr key={item.group_email}>
                  <td>{item.group_email}</td>
                  <td>{item.role}</td>
                  <td>{item.created_at || '-'}</td>
                  <td className="actions-cell">
                    <button onClick={() => onEdit(item)}>Edit</button>
                    <button className="danger" onClick={() => onDelete(item.group_email)}>Delete</button>
                  </td>
                </tr>
              ))}
              {mappings.length === 0 ? (
                <tr>
                  <td colSpan="4" className="small-hint">No Google group mappings configured.</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

export function OIDCConfigModal({ open, config, mode = 'oidc', onClose, onChange, onTest, onSave }) {
  if (!open) return null;
  const isEntra = mode === 'entra';
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card google-auth-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>{isEntra ? 'Configure Azure Entra ID' : 'Configure OpenID Connect'}</h3>
          <button className="modal-close" type="button" aria-label="Close" onClick={onClose}>×</button>
        </div>
        <p className="modal-muted">
          {isEntra
            ? 'Configure Azure Entra ID sign-in. BeaverDeck resolves group mappings from token group claims and Microsoft Graph when Graph scopes are granted.'
            : 'Configure a standard OpenID Connect provider using discovery and map roles from the configured groups claim.'}
        </p>
        <div className="settings-form">
          <label className="settings-form-row">
            <span className="settings-form-label">Provider Name</span>
            <input
              value={config.provider_name}
              onChange={(e) => onChange('provider_name', e.target.value)}
              placeholder={isEntra ? 'Azure Entra ID' : 'OpenID Connect'}
            />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Issuer URL</span>
            <input
              value={config.issuer_url}
              onChange={(e) => onChange('issuer_url', e.target.value)}
              placeholder={isEntra ? 'https://login.microsoftonline.com/<tenant-id>/v2.0' : 'issuer url'}
            />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Client ID</span>
            <input value={config.client_id} onChange={(e) => onChange('client_id', e.target.value)} placeholder="client id" />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Client Secret</span>
            <PasswordField value={config.client_secret} onChange={(e) => onChange('client_secret', e.target.value)} placeholder="client secret" />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Scopes</span>
            <input
              value={config.scopes}
              onChange={(e) => onChange('scopes', e.target.value)}
              placeholder={isEntra ? 'openid email profile User.Read GroupMember.Read.All' : 'openid email profile groups'}
            />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Hosted Domain</span>
            <input value={config.hosted_domain} onChange={(e) => onChange('hosted_domain', e.target.value)} placeholder="hosted domain (optional)" />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Email Claim</span>
            <input value={config.email_claim} onChange={(e) => onChange('email_claim', e.target.value)} placeholder={isEntra ? 'email' : 'email claim'} />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Groups Claim</span>
            <input value={config.groups_claim} onChange={(e) => onChange('groups_claim', e.target.value)} placeholder={isEntra ? 'groups' : 'groups claim'} />
          </label>
        </div>
        <div className="modal-actions">
          <button className="secondary" onClick={onTest}>Test</button>
          <button onClick={onSave}>Save</button>
        </div>
      </div>
    </div>
  );
}

export function OIDCMappingsModal({
  open,
  onClose,
  providerName,
  mode = 'oidc',
  mappings,
  groupName,
  role,
  editingGroupName,
  roles,
  onGroupNameChange,
  onRoleChange,
  onSave,
  onCancel,
  onEdit,
  onDelete
}) {
  if (!open) return null;
  const isEntra = mode === 'entra';
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card google-auth-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Configure {providerName} Group Mapping</h3>
          <button className="modal-close" type="button" aria-label="Close" onClick={onClose}>×</button>
        </div>
        <p className="modal-muted">
          {isEntra
            ? 'Map Azure Entra ID groups to BeaverDeck roles. Use the group object ID, display name, mail address, or security identifier returned by Microsoft Graph.'
            : 'Map OIDC group claim values to BeaverDeck roles.'}
        </p>
        <div className="toolbar fixed-toolbar mapping-form">
          <input value={groupName} onChange={(e) => onGroupNameChange(e.target.value)} placeholder={isEntra ? 'group object id or display name' : 'group claim value'} disabled={Boolean(editingGroupName)} />
          <select value={role} onChange={(e) => onRoleChange(e.target.value)}>
            {roles.map((item) => (
              <option key={item.name} value={item.name}>{item.name}</option>
            ))}
          </select>
          <button onClick={onSave}>{editingGroupName ? 'Update' : 'Add'}</button>
          {editingGroupName ? <button className="secondary" onClick={onCancel}>Cancel</button> : null}
        </div>
        {editingGroupName ? <div className="small-hint">Editing mapping for {editingGroupName}</div> : null}
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Group</th>
                <th>Role</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {mappings.map((item) => (
                <tr key={item.group_name}>
                  <td>{item.group_name}</td>
                  <td>{item.role}</td>
                  <td>{item.created_at || '-'}</td>
                  <td className="actions-cell">
                    <button onClick={() => onEdit(item)}>Edit</button>
                    <button className="danger" onClick={() => onDelete(item.group_name)}>Delete</button>
                  </td>
                </tr>
              ))}
              {mappings.length === 0 ? (
                <tr>
                  <td colSpan="4" className="small-hint">No {isEntra ? 'Azure Entra ID' : 'OIDC'} group mappings configured.</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

export function RoleModal({
  open,
  onClose,
  editingRoleName,
  roleFormName,
  roleFormMode,
  roleFormNamespaces,
  roleFormPermissions,
  namespaces,
  roleResources,
  rolesOptionsFor,
  resolveRoleLevel,
  permissionLevel,
  setRoleFormName,
  setRoleFormMode,
  onSelectAllNamespaces,
  onClearNamespaces,
  toggleRoleNamespace,
  setRolePermissionLevel,
  setAllRolePermissionLevels,
  onSave
}) {
  if (!open) return null;
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card role-modal" onClick={(e) => e.stopPropagation()}>
        <h3>{editingRoleName ? `Edit Role: ${editingRoleName}` : 'Create Role'}</h3>
        <p className="modal-muted">Configure namespaces and per-resource access.</p>

        <div className="modal-grid">
          <div>
            <div className="small-label">Role Name</div>
            <input value={roleFormName} onChange={(e) => setRoleFormName(e.target.value)} placeholder="role name" disabled={Boolean(editingRoleName)} />
          </div>
          <div className="role-admin-field">
            <div className="small-label">Privileges</div>
            <label className="role-toggle-row">
              <input type="checkbox" checked={roleFormMode === 'admin'} onChange={(e) => setRoleFormMode(e.target.checked ? 'admin' : 'viewer')} />
              <span>Is Admin</span>
              <span className="info-badge" data-tooltip="Admin bypasses namespace and resource restrictions. The role can access every namespace and all product features.">i</span>
            </label>
          </div>
        </div>

        {roleFormMode !== 'admin' && (
          <>
            <div className="small-label">Visible Namespaces (empty = all)</div>
            <div className="role-ns-panel">
              <div className="role-ns-summary">
                {roleFormNamespaces.length === 0 ? 'All namespaces' : `${roleFormNamespaces.length} selected`}
              </div>
              <div className="ns-picker-actions role-ns-actions">
                <button type="button" onClick={onSelectAllNamespaces}>All</button>
                <button type="button" onClick={onClearNamespaces}>Clear</button>
              </div>
              <div className="role-ns-list">
                {namespaces.map((ns) => (
                  <label key={ns} className="role-ns-item">
                    <input type="checkbox" checked={roleFormNamespaces.includes(ns)} onChange={() => toggleRoleNamespace(ns)} />
                    <span>{ns}</span>
                  </label>
                ))}
              </div>
            </div>

            <div className="small-label">Resource Permissions</div>
            <div className="role-perm-toolbar">
              <span className="role-perm-toolbar-label">Quick set:</span>
              <button type="button" className="perm-all-btn" onClick={() => setAllRolePermissionLevels('none')}>No access</button>
              <button type="button" className="perm-all-btn" onClick={() => setAllRolePermissionLevels('view')}>View</button>
              <button type="button" className="perm-all-btn" onClick={() => setAllRolePermissionLevels('edit')}>Manage</button>
              <button type="button" className="perm-all-btn" onClick={() => setAllRolePermissionLevels('full')}>Full</button>
            </div>
            <div className="table-wrap role-perm-wrap">
              <table className="role-access-table">
                <thead>
                  <tr>
                    <th>Resource</th>
                    <th>Access</th>
                    <th>Meaning</th>
                  </tr>
                </thead>
                <tbody>
                  {roleResources.map((resource) => {
                    const options = rolesOptionsFor(resource);
                    const currentLevel = resolveRoleLevel(resource, permissionLevel(roleFormPermissions[resource]));
                    const currentOption = options.find((option) => option.value === currentLevel) || options[0];
                    return (
                      <tr key={resource}>
                        <td>{resource}</td>
                        <td className="perm-level-cell">
                          <select value={currentLevel} onChange={(e) => setRolePermissionLevel(resource, e.target.value)}>
                            {options.map((option) => (
                              <option key={option.value} value={option.value}>{option.label}</option>
                            ))}
                          </select>
                        </td>
                        <td className="role-access-hint">{currentOption?.hint || '-'}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </>
        )}

        <div className="modal-actions">
          <button onClick={onClose}>Cancel</button>
          <button onClick={onSave}>{editingRoleName ? 'Save Role' : 'Create Role'}</button>
        </div>
      </div>
    </div>
  );
}

export function CreateUserModal({
  open,
  onClose,
  username,
  setUsername,
  password,
  setPassword,
  role,
  setRole,
  roles,
  onSubmit
}) {
  if (!open) return null;
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Create Local User</h3>
          <button className="modal-close" type="button" aria-label="Close" onClick={onClose}>×</button>
        </div>
        <p className="modal-muted">Create an internal BeaverDeck user with a local password.</p>
        <div className="settings-form">
          <label className="settings-form-row">
            <span className="settings-form-label">Username</span>
            <input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="username" />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Password</span>
            <PasswordField value={password} onChange={(e) => setPassword(e.target.value)} placeholder="password" />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Role</span>
            <select value={role} onChange={(e) => setRole(e.target.value)}>
              {roles.map((item) => (
                <option key={item.name} value={item.name}>{item.name}</option>
              ))}
            </select>
          </label>
        </div>
        <div className="modal-actions">
          <button className="secondary" onClick={onClose}>Cancel</button>
          <button onClick={onSubmit}>Create User</button>
        </div>
      </div>
    </div>
  );
}

export function PasswordPromptModal({
  open,
  title,
  subjectLabel,
  subjectValue,
  password,
  setPassword,
  confirmPassword,
  setConfirmPassword,
  onClose,
  onSubmit
}) {
  if (!open) return null;
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>{title}</h3>
          <button className="modal-close" type="button" aria-label="Close" onClick={onClose}>×</button>
        </div>
        {subjectValue ? (
          <div className="modal-muted">{subjectLabel}: {subjectValue}</div>
        ) : null}
        <div className="settings-form">
          <label className="settings-form-row">
            <span className="settings-form-label">New Password</span>
            <PasswordField value={password} onChange={(e) => setPassword(e.target.value)} placeholder="new password" />
          </label>
          <label className="settings-form-row">
            <span className="settings-form-label">Confirm Password</span>
            <PasswordField value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} placeholder="confirm password" />
          </label>
        </div>
        <div className="modal-actions">
          <button className="secondary" onClick={onClose}>Cancel</button>
          <button onClick={onSubmit}>Apply</button>
        </div>
      </div>
    </div>
  );
}

export function ProfileModal({ open, onClose, currentUser, selectedNamespaces, themeOptions, themePreference, resolvedTheme, onThemeChange, onLogout }) {
  if (!open) return null;
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <h3>Profile</h3>
        <p className="modal-muted">User settings.</p>
        <div className="modal-grid">
          <div>
            <div className="small-label">User</div>
            <div>{currentUser.username || '-'}</div>
          </div>
          <div>
            <div className="small-label">Role</div>
            <div>{currentUser.role || '-'}</div>
          </div>
          <div>
            <div className="small-label">Auth Source</div>
            <div>{currentUser.authSource || '-'}</div>
          </div>
          <div>
            <div className="small-label">Current Namespace</div>
            <div>{selectedNamespaces.join(', ') || '-'}</div>
          </div>
          <div>
            <div className="small-label">Application Version</div>
            <div>{currentUser.appVersion || '-'}</div>
          </div>
          <div>
            <div className="small-label">Latest Version</div>
            <div>{currentUser.latestVersion || '-'}</div>
          </div>
          <div>
            <div className="small-label">Theme</div>
            <div className="theme-choice" role="radiogroup" aria-label="Theme">
              {themeOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  className={themePreference === option.value ? 'active' : ''}
                  onClick={() => onThemeChange(option.value)}
                  role="radio"
                  aria-checked={themePreference === option.value}
                >
                  {option.label}
                </button>
              ))}
            </div>
            <div className="small-hint modal-hint">Active theme: {resolvedTheme === 'dark' ? 'Dark' : 'Light'}</div>
          </div>
        </div>
        <div className="modal-actions">
          <button onClick={onClose}>Close</button>
          <button className="danger" onClick={onLogout}>Logout</button>
        </div>
      </div>
    </div>
  );
}

export function DrainModal({ open, onClose, drainTargetNode, drainForce, onForceChange, onDrain }) {
  if (!open) return null;
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card drain-modal" onClick={(e) => e.stopPropagation()}>
        <h3>Drain Node: {drainTargetNode}</h3>
        <p className="modal-muted">Cordon the node and evict pods that are safe to move away.</p>
        <div className="small-label">What happens</div>
        <div className="small-hint modal-hint">
          Drain marks the node unschedulable and evicts normal workload pods. DaemonSet pods and static/mirror pods stay in place.
        </div>
        <label className="toggle-row modal-toggle-row">
          <input type="checkbox" checked={drainForce} onChange={(e) => onForceChange(e.target.checked)} />
          <span>Force</span>
        </label>
        <div className="small-hint modal-hint">
          Force also includes unmanaged pods and pods with local storage. It still does not evict DaemonSet pods or static/mirror pods.
        </div>
        <div className="modal-actions">
          <button onClick={onClose}>Cancel</button>
          <button className="warn" onClick={onDrain}>Drain</button>
        </div>
      </div>
    </div>
  );
}

export function ScaleModal({ open, onClose, scaleTargetKind, deploymentNamespace, deploymentName, replicas, onReplicasChange, canApply, applyReason, onApply }) {
  if (!open) return null;
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card scale-modal" onClick={(e) => e.stopPropagation()}>
        <h3>Scale Workload</h3>
        <p className="modal-muted">Choose the desired replica count for the selected workload.</p>
        <div className="modal-grid">
          <div>
            <div className="small-label">Workload</div>
            <div>{deploymentNamespace ? `${scaleTargetKind} ${deploymentNamespace}/${deploymentName}` : deploymentName || '-'}</div>
          </div>
          <div>
            <div className="small-label">Replicas</div>
            <input type="number" min="0" step="1" value={replicas} onChange={(e) => onReplicasChange(Math.max(0, Number(e.target.value) || 0))} />
          </div>
        </div>
        <div className="modal-actions">
          <button onClick={onClose}>Cancel</button>
          <button onClick={onApply} disabled={!canApply} title={applyReason}>Apply</button>
        </div>
      </div>
    </div>
  );
}
