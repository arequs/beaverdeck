import React from 'react';
import ActionMenu from './ActionMenu.jsx';

export default function UserManagementPage({
  managedUsers,
  managedRoles,
  configImportInputRef,
  exportAdminConfig,
  openConfigImportPicker,
  importAdminConfigFile,
  openCreateUserModal,
  updateUserRole,
  resetLocalUserPassword,
  deleteUser,
  openCreateRoleModal,
  normalizeRolePermissions,
  openEditRoleModal,
  deleteRole,
  googleAuthConfigured,
  googleConfig,
  googleMappings,
  setShowGoogleConfigModal,
  setShowGoogleMappingsModal,
  disableGoogleAuth,
  oidcAuthConfigured,
  oidcConfig,
  oidcMappings,
  openOIDCConfigModal,
  openEntraConfigModal,
  setShowOIDCMappingsModal,
  disableOIDCAuth,
  safe
}) {
  const providerName = oidcConfig.provider_name || 'OpenID Connect';
  const isEntraConfigured = /entra|azure|microsoftonline\.com|sts\.windows\.net/i.test(`${oidcConfig.provider_name || ''} ${oidcConfig.issuer_url || ''}`);
  const genericOIDCConfigured = oidcAuthConfigured && !isEntraConfigured;

  return (
    <div className="admin-sections">
      <section className="admin-section admin-section-prominent">
        <div className="admin-section-header">
          <div>
            <div className="small-label">Configuration Backup</div>
            <div className="admin-section-description">Export or import BeaverDeck auth configuration, roles, mappings and local users.</div>
          </div>
          <div className="toolbar fixed-toolbar admin-section-actions">
            <button onClick={() => safe(exportAdminConfig)}>Export Configuration</button>
            <button className="secondary" onClick={openConfigImportPicker}>Import Configuration</button>
            <input
              ref={configImportInputRef}
              type="file"
              accept="application/yaml,text/yaml,.yaml,.yml"
              hidden
              onChange={(event) => safe(() => importAdminConfigFile(event.target.files?.[0]))}
            />
          </div>
        </div>
      </section>

      <section className="admin-section admin-section-prominent admin-section-configured">
        <div className="admin-section-header">
          <div>
            <div className="small-label">Local Users and Roles</div>
            <div className="admin-section-description">Manage internal BeaverDeck users, passwords and RBAC roles.</div>
          </div>
          <div className="toolbar fixed-toolbar admin-section-actions">
            <button onClick={openCreateUserModal}>Create User</button>
            <button onClick={openCreateRoleModal}>Create Role</button>
          </div>
        </div>

        <div className="small-label">Users</div>
        <div className="table-wrap admin-table-wrap">
          <table className="admin-users-table">
            <thead>
              <tr>
                <th>Username</th>
                <th>Source</th>
                <th>Role</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {managedUsers.map((u) => (
                <tr key={u.username}>
                  <td>{u.username}</td>
                  <td>{u.auth_source}</td>
                  <td>{u.role}</td>
                  <td>{u.created_at || '-'}</td>
                  <td className="actions-cell">
                    <select
                      className="admin-user-role-select"
                      value={u.role}
                      onChange={(e) => safe(() => updateUserRole(u.username, e.target.value))}
                      disabled={u.auth_source !== 'local'}
                      title={u.auth_source !== 'local' ? `${u.auth_source} user role is managed by external group mapping` : ''}
                    >
                      {managedRoles.map((r) => (
                        <option key={r.name} value={r.name}>{r.name}</option>
                      ))}
                    </select>
                    <ActionMenu
                      actions={[
                        ...(u.auth_source === 'local' ? [{
                          label: 'Reset password',
                          enabled: true,
                          onClick: () => safe(() => resetLocalUserPassword(u.username))
                        }] : []),
                        {
                          label: 'Delete',
                          enabled: true,
                          onClick: () => safe(() => deleteUser(u.username))
                        }
                      ]}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="small-label">Roles</div>
        <div className="table-wrap admin-table-wrap">
          <table>
            <thead>
              <tr>
                <th>Role</th>
                <th>Admin</th>
                <th>Created</th>
                <th>Namespaces</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {managedRoles.map((r) => (
                <tr key={r.name}>
                  <td>{r.name}</td>
                  <td>{r.mode === 'admin' ? 'yes' : 'no'}</td>
                  <td>{r.created_at || '-'}</td>
                  <td>
                    {(normalizeRolePermissions(r.permissions).namespaces || []).length
                      ? normalizeRolePermissions(r.permissions).namespaces.join(', ')
                      : 'all'}
                  </td>
                  <td className="actions-cell">
                    <button onClick={() => openEditRoleModal(r)}>Edit</button>
                    <button className="danger" onClick={() => safe(() => deleteRole(r.name))} disabled={r.name === 'admin'}>
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className={`admin-section admin-section-prominent ${googleAuthConfigured ? 'admin-section-configured' : ''}`.trim()}>
        <div className="admin-section-header">
          <div>
            <div className="small-label">Google OAuth</div>
            <div className="admin-section-description">Configure Google sign-in and map Google Workspace groups to BeaverDeck roles.</div>
          </div>
        </div>
        <div className="cluster-card google-auth-panel">
          <div className="google-auth-status-row">
            <div>
              <div className="small-label">Status</div>
              <div className={`google-auth-badge ${googleAuthConfigured ? 'configured' : 'disabled'}`}>
                {googleAuthConfigured ? 'Configured' : 'Not configured'}
              </div>
            </div>
            <div>
              <div className="small-label">Hosted Domain</div>
              <div>{googleConfig.hosted_domain || '-'}</div>
            </div>
            <div>
              <div className="small-label">Delegated Admin</div>
              <div>{googleConfig.delegated_admin_email || '-'}</div>
            </div>
            <div>
              <div className="small-label">Group Mappings</div>
              <div>{googleMappings.length}</div>
            </div>
          </div>
          <div className="toolbar fixed-toolbar">
            <button onClick={() => setShowGoogleConfigModal(true)}>Configure</button>
            <button className="secondary" onClick={() => setShowGoogleMappingsModal(true)}>Configure Group Mapping</button>
            <button className="danger" onClick={() => safe(disableGoogleAuth)}>Disable</button>
          </div>
        </div>
        <div className="small-hint">
          Google sign-in button is shown only when the full Google auth configuration is present.
        </div>
        <div className="small-label">Google Group Role Mapping</div>
        <div className="table-wrap admin-table-wrap">
          <table>
            <thead>
              <tr>
                <th>Google Group</th>
                <th>Role</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {googleMappings.map((item) => (
                <tr key={item.group_email}>
                  <td>{item.group_email}</td>
                  <td>{item.role}</td>
                  <td>{item.created_at || '-'}</td>
                </tr>
              ))}
              {googleMappings.length === 0 ? (
                <tr>
                  <td colSpan="3" className="small-hint">No Google group mappings configured.</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <section className={`admin-section admin-section-prominent ${genericOIDCConfigured ? 'admin-section-configured' : ''}`.trim()}>
        <div className="admin-section-header">
          <div>
            <div className="small-label">OpenID Connect</div>
            <div className="admin-section-description">Configure a generic OpenID Connect provider and map external group claims to BeaverDeck roles.</div>
          </div>
        </div>
        <div className="cluster-card google-auth-panel">
          <div className="google-auth-status-row">
            <div>
              <div className="small-label">Status</div>
              <div className={`google-auth-badge ${genericOIDCConfigured ? 'configured' : 'disabled'}`}>
                {genericOIDCConfigured ? 'Configured' : 'Not configured'}
              </div>
            </div>
            <div>
              <div className="small-label">Issuer</div>
              <div>{genericOIDCConfigured ? (oidcConfig.issuer_url || '-') : '-'}</div>
            </div>
            <div>
              <div className="small-label">Hosted Domain</div>
              <div>{genericOIDCConfigured ? (oidcConfig.hosted_domain || '-') : '-'}</div>
            </div>
            <div>
              <div className="small-label">Group Mappings</div>
              <div>{genericOIDCConfigured ? oidcMappings.length : 0}</div>
            </div>
          </div>
          <div className="toolbar fixed-toolbar">
            <button onClick={openOIDCConfigModal}>Configure</button>
            <button className="secondary" onClick={() => setShowOIDCMappingsModal(true)}>Configure Group Mapping</button>
            <button className="danger" onClick={() => safe(disableOIDCAuth)}>Disable</button>
          </div>
        </div>
        <div className="small-hint">
          Generic OIDC uses discovery and group mapping via the configured groups claim.
        </div>
        <div className="small-label">OIDC Group Mapping</div>
        <div className="table-wrap admin-table-wrap">
          <table>
            <thead>
              <tr>
                <th>Group</th>
                <th>Role</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {genericOIDCConfigured ? oidcMappings.map((item) => (
                <tr key={item.group_name}>
                  <td>{item.group_name}</td>
                  <td>{item.role}</td>
                  <td>{item.created_at || '-'}</td>
                </tr>
              )) : null}
              {!genericOIDCConfigured || oidcMappings.length === 0 ? (
                <tr>
                  <td colSpan="3" className="small-hint">No OIDC group mappings configured.</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <section className={`admin-section admin-section-prominent ${oidcAuthConfigured && isEntraConfigured ? 'admin-section-configured' : ''}`.trim()}>
        <div className="admin-section-header">
          <div>
            <div className="small-label">Azure Entra ID</div>
            <div className="admin-section-description">Configure Microsoft Entra sign-in and resolve role mappings from Entra groups.</div>
          </div>
        </div>
        <div className="cluster-card google-auth-panel">
          <div className="google-auth-status-row">
            <div>
              <div className="small-label">Status</div>
              <div className={`google-auth-badge ${oidcAuthConfigured && isEntraConfigured ? 'configured' : 'disabled'}`}>
                {oidcAuthConfigured && isEntraConfigured ? 'Configured' : 'Not configured'}
              </div>
            </div>
            <div>
              <div className="small-label">Issuer</div>
              <div>{isEntraConfigured ? (oidcConfig.issuer_url || '-') : '-'}</div>
            </div>
            <div>
              <div className="small-label">Graph Lookup</div>
              <div>{isEntraConfigured && /GroupMember\.Read\.All/i.test(oidcConfig.scopes || '') ? 'enabled' : 'scope required'}</div>
            </div>
            <div>
              <div className="small-label">Group Mappings</div>
              <div>{isEntraConfigured ? oidcMappings.length : 0}</div>
            </div>
          </div>
          <div className="toolbar fixed-toolbar">
            <button onClick={openEntraConfigModal}>Configure Entra ID</button>
            <button className="secondary" onClick={() => setShowOIDCMappingsModal(true)}>Configure Group Mapping</button>
            <button className="danger" onClick={() => safe(disableOIDCAuth)}>Disable</button>
          </div>
        </div>
        <div className="small-hint">
          Entra ID uses standard OIDC login plus Microsoft Graph group lookup when scopes include User.Read and GroupMember.Read.All.
        </div>
        <div className="small-label">{isEntraConfigured ? providerName : 'Azure Entra ID'} Group Mapping</div>
        <div className="table-wrap admin-table-wrap">
          <table>
            <thead>
              <tr>
                <th>Group</th>
                <th>Role</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {isEntraConfigured ? oidcMappings.map((item) => (
                <tr key={item.group_name}>
                  <td>{item.group_name}</td>
                  <td>{item.role}</td>
                  <td>{item.created_at || '-'}</td>
                </tr>
              )) : null}
              {!isEntraConfigured || oidcMappings.length === 0 ? (
                <tr>
                  <td colSpan="3" className="small-hint">No Azure Entra ID group mappings configured.</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
