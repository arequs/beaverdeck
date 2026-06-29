import React from 'react';
import ActionMenu from './ActionMenu.jsx';

export function ClusterRolesPage({
  sortedClusterRoles,
  sortedClusterRoleBindings,
  toggleSort,
  sortMark,
  makeAction,
  permissionInfo,
  primaryNamespace,
  safe,
  openManifestTab,
  allAllowed,
  openEditTab,
  deleteResourceByRef,
  refreshAll
}) {
  return (
    <div className="stacked-table-view">
      <section className="stacked-table-section">
        <div className="small-label">ClusterRoles</div>
        <div className="table-wrap stacked-table-wrap">
          <table>
            <thead>
              <tr>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'name')}>Name {sortMark('clusterroles', 'name')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'rules')}>Rules {sortMark('clusterroles', 'rules')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'age')}>Age {sortMark('clusterroles', 'age')}</button></th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {sortedClusterRoles.map((item) => (
                <tr key={item.name}>
                  <td>{item.name}</td>
                  <td>{item.rules}</td>
                  <td>{item.age}</td>
                  <td className="actions-cell">
                    <ActionMenu
                      actions={[
                        makeAction('Manifest', permissionInfo('clusterroles', 'view'), () => safe(() => openManifestTab(primaryNamespace, 'clusterrole', item.name))),
                        makeAction('Edit', permissionInfo('clusterroles', 'edit'), () => safe(() => openEditTab(primaryNamespace, 'clusterrole', item.name))),
                        makeAction('Delete', permissionInfo('clusterroles', 'delete'), () => safe(async () => {
                          await deleteResourceByRef('clusterrole', '', item.name);
                          await refreshAll();
                        }))
                      ]}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="stacked-table-section">
        <div className="small-label">ClusterRoleBindings</div>
        <div className="table-wrap stacked-table-wrap">
          <table>
            <thead>
              <tr>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'name')}>Binding {sortMark('clusterroles', 'name')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'role_ref')}>Role Ref {sortMark('clusterroles', 'role_ref')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'subjects')}>Subjects {sortMark('clusterroles', 'subjects')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'age')}>Age {sortMark('clusterroles', 'age')}</button></th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {sortedClusterRoleBindings.map((item) => (
                <tr key={item.name}>
                  <td>{item.name}</td>
                  <td>{item.role_ref}</td>
                  <td>{item.subjects}</td>
                  <td>{item.age}</td>
                  <td className="actions-cell">
                    <ActionMenu
                      actions={[
                        makeAction('Manifest', permissionInfo('clusterroles', 'view'), () => safe(() => openManifestTab(primaryNamespace, 'clusterrolebinding', item.name))),
                        makeAction('Edit', permissionInfo('clusterroles', 'edit'), () => safe(() => openEditTab(primaryNamespace, 'clusterrolebinding', item.name))),
                        makeAction('Delete', permissionInfo('clusterroles', 'delete'), () => safe(async () => {
                          await deleteResourceByRef('clusterrolebinding', '', item.name);
                          await refreshAll();
                        }))
                      ]}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

export function NamespacedRolesPage({
  sortedRbacRoles,
  sortedRoleBindings,
  toggleSort,
  sortMark,
  makeAction,
  permissionInfo,
  safe,
  openManifestTab,
  allAllowed,
  openEditTab,
  deleteResourceByRef,
  refreshAll
}) {
  return (
    <div className="stacked-table-view">
      <section className="stacked-table-section">
        <div className="small-label">Roles</div>
        <div className="table-wrap stacked-table-wrap">
          <table>
            <thead>
              <tr>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'name')}>Name {sortMark('rbacroles', 'name')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'namespace')}>Namespace {sortMark('rbacroles', 'namespace')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'rules')}>Rules {sortMark('rbacroles', 'rules')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'age')}>Age {sortMark('rbacroles', 'age')}</button></th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {sortedRbacRoles.map((item) => (
                <tr key={`${item.namespace}/${item.name}`}>
                  <td>{item.name}</td>
                  <td>{item.namespace}</td>
                  <td>{item.rules}</td>
                  <td>{item.age}</td>
                  <td className="actions-cell">
                    <ActionMenu
                      actions={[
                        makeAction('Manifest', permissionInfo('rbacroles', 'view', item.namespace), () => safe(() => openManifestTab(item.namespace, 'role', item.name))),
                        makeAction('Edit', permissionInfo('rbacroles', 'edit', item.namespace), () => safe(() => openEditTab(item.namespace, 'role', item.name))),
                        makeAction('Delete', permissionInfo('rbacroles', 'delete', item.namespace), () => safe(async () => {
                          await deleteResourceByRef('role', item.namespace, item.name);
                          await refreshAll();
                        }))
                      ]}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="stacked-table-section">
        <div className="small-label">RoleBindings</div>
        <div className="table-wrap stacked-table-wrap">
          <table>
            <thead>
              <tr>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'name')}>Binding {sortMark('rbacroles', 'name')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'namespace')}>Namespace {sortMark('rbacroles', 'namespace')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'role_ref')}>Role Ref {sortMark('rbacroles', 'role_ref')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'subjects')}>Subjects {sortMark('rbacroles', 'subjects')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'age')}>Age {sortMark('rbacroles', 'age')}</button></th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {sortedRoleBindings.map((item) => (
                <tr key={`${item.namespace}/${item.name}`}>
                  <td>{item.name}</td>
                  <td>{item.namespace}</td>
                  <td>{item.role_ref}</td>
                  <td>{item.subjects}</td>
                  <td>{item.age}</td>
                  <td className="actions-cell">
                    <ActionMenu
                      actions={[
                        makeAction('Manifest', permissionInfo('rbacroles', 'view', item.namespace), () => safe(() => openManifestTab(item.namespace, 'rolebinding', item.name))),
                        makeAction('Edit', permissionInfo('rbacroles', 'edit', item.namespace), () => safe(() => openEditTab(item.namespace, 'rolebinding', item.name))),
                        makeAction('Delete', permissionInfo('rbacroles', 'delete', item.namespace), () => safe(async () => {
                          await deleteResourceByRef('rolebinding', item.namespace, item.name);
                          await refreshAll();
                        }))
                      ]}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

export function ServiceAccountsPage({
  serviceAccountSearch,
  setServiceAccountSearch,
  sortedServiceAccounts,
  toggleSort,
  sortMark,
  makeAction,
  permissionInfo,
  safe,
  openManifestTab,
  allAllowed,
  openEditTab,
  deleteResourceByRef,
  refreshAll
}) {
  return (
    <>
    <div className="toolbar fixed-toolbar">
      <input value={serviceAccountSearch} onChange={(e) => setServiceAccountSearch(e.target.value)} placeholder="Search service accounts..." />
    </div>
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th><button className="sort-btn" onClick={() => toggleSort('serviceaccounts', 'name')}>Name {sortMark('serviceaccounts', 'name')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('serviceaccounts', 'namespace')}>Namespace {sortMark('serviceaccounts', 'namespace')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('serviceaccounts', 'secrets')}>Secrets {sortMark('serviceaccounts', 'secrets')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('serviceaccounts', 'age')}>Age {sortMark('serviceaccounts', 'age')}</button></th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedServiceAccounts.map((item) => (
            <tr key={`${item.namespace}/${item.name}`}>
              <td>{item.name}</td>
              <td>{item.namespace}</td>
              <td>{item.secrets}</td>
              <td>{item.age}</td>
              <td className="actions-cell">
                <ActionMenu
                  actions={[
                    makeAction('Manifest', permissionInfo('serviceaccounts', 'view', item.namespace), () => safe(() => openManifestTab(item.namespace, 'serviceaccount', item.name))),
                    makeAction('Edit', permissionInfo('serviceaccounts', 'edit', item.namespace), () => safe(() => openEditTab(item.namespace, 'serviceaccount', item.name))),
                    makeAction('Delete', permissionInfo('serviceaccounts', 'delete', item.namespace), () => safe(async () => {
                      await deleteResourceByRef('serviceaccount', item.namespace, item.name);
                      await refreshAll();
                    }))
                  ]}
                />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
    </>
  );
}
