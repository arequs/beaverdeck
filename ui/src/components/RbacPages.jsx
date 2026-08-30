import React from 'react';
import ActionMenu from './ActionMenu.jsx';
import { BulkActionButton, BulkRowSelect, BulkSelectAll, BulkSelectionCount } from './BulkSelection.jsx';
import useBulkSelection from '../hooks/useBulkSelection.js';

function serviceAccountKey(item) {
  return `${item.namespace}/${item.name}`;
}

function clusterResourceKey(item) {
  return item.name;
}

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
  const rolesSelection = useBulkSelection(sortedClusterRoles, clusterResourceKey);
  const bindingsSelection = useBulkSelection(sortedClusterRoleBindings, clusterResourceKey);
  return (
    <div className="stacked-table-view">
      <section className="stacked-table-section">
        <div className="toolbar fixed-toolbar">
          <div className="small-label">ClusterRoles</div>
          <BulkSelectionCount selection={rolesSelection} />
          {rolesSelection.count > 0 ? (
            <BulkActionButton
              selection={rolesSelection}
              verb="Delete"
              className="danger"
              getPermission={() => permissionInfo('clusterroles', 'delete')}
              runItem={(item) => deleteResourceByRef('clusterrole', '', item.name)}
              refreshAll={refreshAll}
              safe={safe}
            />
          ) : null}
        </div>
        <div className="table-wrap stacked-table-wrap">
          <table>
            <thead>
              <tr>
                <th><BulkSelectAll selection={rolesSelection} label="cluster roles" /></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'name')}>Name {sortMark('clusterroles', 'name')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'rules')}>Rules {sortMark('clusterroles', 'rules')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'age')}>Age {sortMark('clusterroles', 'age')}</button></th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {sortedClusterRoles.map((item) => (
                <tr key={item.name} className={rolesSelection.selectedKeySet.has(clusterResourceKey(item)) ? 'active-row' : ''}>
                  <td><BulkRowSelect selection={rolesSelection} item={item} itemKey={clusterResourceKey(item)} label={item.name} /></td>
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
        <div className="toolbar fixed-toolbar">
          <div className="small-label">ClusterRoleBindings</div>
          <BulkSelectionCount selection={bindingsSelection} />
          {bindingsSelection.count > 0 ? (
            <BulkActionButton
              selection={bindingsSelection}
              verb="Delete"
              className="danger"
              getPermission={() => permissionInfo('clusterroles', 'delete')}
              runItem={(item) => deleteResourceByRef('clusterrolebinding', '', item.name)}
              refreshAll={refreshAll}
              safe={safe}
            />
          ) : null}
        </div>
        <div className="table-wrap stacked-table-wrap">
          <table>
            <thead>
              <tr>
                <th><BulkSelectAll selection={bindingsSelection} label="cluster role bindings" /></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'name')}>Binding {sortMark('clusterroles', 'name')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'role_ref')}>Role Ref {sortMark('clusterroles', 'role_ref')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'subjects')}>Subjects {sortMark('clusterroles', 'subjects')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('clusterroles', 'age')}>Age {sortMark('clusterroles', 'age')}</button></th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {sortedClusterRoleBindings.map((item) => (
                <tr key={item.name} className={bindingsSelection.selectedKeySet.has(clusterResourceKey(item)) ? 'active-row' : ''}>
                  <td><BulkRowSelect selection={bindingsSelection} item={item} itemKey={clusterResourceKey(item)} label={item.name} /></td>
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
  const rolesSelection = useBulkSelection(sortedRbacRoles, serviceAccountKey);
  const bindingsSelection = useBulkSelection(sortedRoleBindings, serviceAccountKey);
  return (
    <div className="stacked-table-view">
      <section className="stacked-table-section">
        <div className="toolbar fixed-toolbar">
          <div className="small-label">Roles</div>
          <BulkSelectionCount selection={rolesSelection} />
          {rolesSelection.count > 0 ? (
            <BulkActionButton
              selection={rolesSelection}
              verb="Delete"
              className="danger"
              getPermission={(item) => permissionInfo('rbacroles', 'delete', item.namespace)}
              runItem={(item) => deleteResourceByRef('role', item.namespace, item.name)}
              refreshAll={refreshAll}
              safe={safe}
            />
          ) : null}
        </div>
        <div className="table-wrap stacked-table-wrap">
          <table>
            <thead>
              <tr>
                <th><BulkSelectAll selection={rolesSelection} label="roles" /></th>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'name')}>Name {sortMark('rbacroles', 'name')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'namespace')}>Namespace {sortMark('rbacroles', 'namespace')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'rules')}>Rules {sortMark('rbacroles', 'rules')}</button></th>
                <th><button className="sort-btn" onClick={() => toggleSort('rbacroles', 'age')}>Age {sortMark('rbacroles', 'age')}</button></th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {sortedRbacRoles.map((item) => (
                <tr key={serviceAccountKey(item)} className={rolesSelection.selectedKeySet.has(serviceAccountKey(item)) ? 'active-row' : ''}>
                  <td><BulkRowSelect selection={rolesSelection} item={item} itemKey={serviceAccountKey(item)} label={`${item.namespace}/${item.name}`} /></td>
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
        <div className="toolbar fixed-toolbar">
          <div className="small-label">RoleBindings</div>
          <BulkSelectionCount selection={bindingsSelection} />
          {bindingsSelection.count > 0 ? (
            <BulkActionButton
              selection={bindingsSelection}
              verb="Delete"
              className="danger"
              getPermission={(item) => permissionInfo('rbacroles', 'delete', item.namespace)}
              runItem={(item) => deleteResourceByRef('rolebinding', item.namespace, item.name)}
              refreshAll={refreshAll}
              safe={safe}
            />
          ) : null}
        </div>
        <div className="table-wrap stacked-table-wrap">
          <table>
            <thead>
              <tr>
                <th><BulkSelectAll selection={bindingsSelection} label="role bindings" /></th>
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
                <tr key={serviceAccountKey(item)} className={bindingsSelection.selectedKeySet.has(serviceAccountKey(item)) ? 'active-row' : ''}>
                  <td><BulkRowSelect selection={bindingsSelection} item={item} itemKey={serviceAccountKey(item)} label={`${item.namespace}/${item.name}`} /></td>
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
  const selection = useBulkSelection(sortedServiceAccounts, serviceAccountKey);
  return (
    <>
    <div className="toolbar fixed-toolbar">
      <BulkSelectionCount selection={selection} />
      {selection.count > 0 ? (
        <BulkActionButton
          selection={selection}
          verb="Delete"
          className="danger"
          getPermission={(item) => permissionInfo('serviceaccounts', 'delete', item.namespace)}
          runItem={(item) => deleteResourceByRef('serviceaccount', item.namespace, item.name)}
          refreshAll={refreshAll}
          safe={safe}
        />
      ) : null}
      <input value={serviceAccountSearch} onChange={(e) => setServiceAccountSearch(e.target.value)} placeholder="Search service accounts..." />
    </div>
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th><BulkSelectAll selection={selection} label="service accounts" /></th>
            <th><button className="sort-btn" onClick={() => toggleSort('serviceaccounts', 'name')}>Name {sortMark('serviceaccounts', 'name')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('serviceaccounts', 'namespace')}>Namespace {sortMark('serviceaccounts', 'namespace')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('serviceaccounts', 'secrets')}>Secrets {sortMark('serviceaccounts', 'secrets')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('serviceaccounts', 'age')}>Age {sortMark('serviceaccounts', 'age')}</button></th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedServiceAccounts.map((item) => (
            <tr key={serviceAccountKey(item)} className={selection.selectedKeySet.has(serviceAccountKey(item)) ? 'active-row' : ''}>
              <td><BulkRowSelect selection={selection} item={item} itemKey={serviceAccountKey(item)} label={`${item.namespace}/${item.name}`} /></td>
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
