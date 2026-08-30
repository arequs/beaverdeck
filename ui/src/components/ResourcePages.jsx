import React from 'react';
import ActionMenu from './ActionMenu.jsx';
import { BulkActionButton, BulkRowSelect, BulkSelectAll, BulkSelectionCount } from './BulkSelection.jsx';
import useBulkSelection from '../hooks/useBulkSelection.js';

function namespacedResourceKey(item) {
  return `${item.namespace}/${item.name}`;
}

function customResourceKey(item) {
  return `${item.namespace || '_cluster'}/${item.name}`;
}

function clusterResourceKey(item) {
  return item.name;
}

export function EventsPage({ sortedEvents }) {
  return <pre className="mono-block">{sortedEvents.map((e) => `${e.last_seen} ns=${e.namespace} ${e.type} ${e.reason} ${e.object}\n${e.message}`).join('\n\n')}</pre>;
}

export function ServicesPage({
  serviceSearch,
  setServiceSearch,
  sortedServices,
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
  const selection = useBulkSelection(sortedServices, namespacedResourceKey);
  return (
    <>
    <div className="toolbar fixed-toolbar">
      <BulkSelectionCount selection={selection} />
      {selection.count > 0 ? (
        <BulkActionButton
          selection={selection}
          verb="Delete"
          className="danger"
          getPermission={(item) => permissionInfo('services', 'delete', item.namespace)}
          runItem={(item) => deleteResourceByRef('service', item.namespace, item.name)}
          refreshAll={refreshAll}
          safe={safe}
        />
      ) : null}
      <input value={serviceSearch} onChange={(e) => setServiceSearch(e.target.value)} placeholder="Search services..." />
    </div>
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th><BulkSelectAll selection={selection} label="services" /></th>
            <th><button className="sort-btn" onClick={() => toggleSort('services', 'name')}>Name {sortMark('services', 'name')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('services', 'namespace')}>Namespace {sortMark('services', 'namespace')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('services', 'type')}>Type {sortMark('services', 'type')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('services', 'cluster_ip')}>ClusterIP {sortMark('services', 'cluster_ip')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('services', 'ports')}>Ports {sortMark('services', 'ports')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('services', 'age')}>Age {sortMark('services', 'age')}</button></th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedServices.map((s) => (
            <tr key={namespacedResourceKey(s)} className={selection.selectedKeySet.has(namespacedResourceKey(s)) ? 'active-row' : ''}>
              <td><BulkRowSelect selection={selection} item={s} itemKey={namespacedResourceKey(s)} label={`${s.namespace}/${s.name}`} /></td>
              <td>{s.name}</td>
              <td>{s.namespace}</td>
              <td>{s.type}</td>
              <td>{s.cluster_ip}</td>
              <td>{s.ports}</td>
              <td>{s.age}</td>
              <td className="actions-cell">
                <ActionMenu
                  actions={[
                    makeAction('Manifest', permissionInfo('services', 'view', s.namespace), () => safe(() => openManifestTab(s.namespace, 'service', s.name))),
                    makeAction('Edit', permissionInfo('services', 'edit', s.namespace), () => safe(() => openEditTab(s.namespace, 'service', s.name))),
                    makeAction('Delete', permissionInfo('services', 'delete', s.namespace), () => safe(async () => {
                      await deleteResourceByRef('service', s.namespace, s.name);
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

export function IngressesPage({
  ingressSearch,
  setIngressSearch,
  sortedIngresses,
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
  const selection = useBulkSelection(sortedIngresses, namespacedResourceKey);
  return (
    <>
    <div className="toolbar fixed-toolbar">
      <BulkSelectionCount selection={selection} />
      {selection.count > 0 ? (
        <BulkActionButton
          selection={selection}
          verb="Delete"
          className="danger"
          getPermission={(item) => permissionInfo('ingresses', 'delete', item.namespace)}
          runItem={(item) => deleteResourceByRef('ingress', item.namespace, item.name)}
          refreshAll={refreshAll}
          safe={safe}
        />
      ) : null}
      <input value={ingressSearch} onChange={(e) => setIngressSearch(e.target.value)} placeholder="Search ingresses..." />
    </div>
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th><BulkSelectAll selection={selection} label="ingresses" /></th>
            <th><button className="sort-btn" onClick={() => toggleSort('ingresses', 'name')}>Name {sortMark('ingresses', 'name')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('ingresses', 'namespace')}>Namespace {sortMark('ingresses', 'namespace')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('ingresses', 'class')}>Class {sortMark('ingresses', 'class')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('ingresses', 'hosts')}>Hosts {sortMark('ingresses', 'hosts')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('ingresses', 'address')}>Address {sortMark('ingresses', 'address')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('ingresses', 'age')}>Age {sortMark('ingresses', 'age')}</button></th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedIngresses.map((i) => (
            <tr key={namespacedResourceKey(i)} className={selection.selectedKeySet.has(namespacedResourceKey(i)) ? 'active-row' : ''}>
              <td><BulkRowSelect selection={selection} item={i} itemKey={namespacedResourceKey(i)} label={`${i.namespace}/${i.name}`} /></td>
              <td>{i.name}</td>
              <td>{i.namespace}</td>
              <td>{i.class}</td>
              <td>{i.hosts}</td>
              <td>{i.address}</td>
              <td>{i.age}</td>
              <td className="actions-cell">
                <ActionMenu
                  actions={[
                    makeAction('Manifest', permissionInfo('ingresses', 'view', i.namespace), () => safe(() => openManifestTab(i.namespace, 'ingress', i.name))),
                    makeAction('Edit', permissionInfo('ingresses', 'edit', i.namespace), () => safe(() => openEditTab(i.namespace, 'ingress', i.name))),
                    makeAction('Delete', permissionInfo('ingresses', 'delete', i.namespace), () => safe(async () => {
                      await deleteResourceByRef('ingress', i.namespace, i.name);
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

export function ConfigMapsPage({
  configMapSearch,
  setConfigMapSearch,
  sortedConfigMaps,
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
  const selection = useBulkSelection(sortedConfigMaps, namespacedResourceKey);
  return (
    <>
    <div className="toolbar fixed-toolbar">
      <BulkSelectionCount selection={selection} />
      {selection.count > 0 ? (
        <BulkActionButton
          selection={selection}
          verb="Delete"
          className="danger"
          getPermission={(item) => permissionInfo('configmaps', 'delete', item.namespace)}
          runItem={(item) => deleteResourceByRef('configmap', item.namespace, item.name)}
          refreshAll={refreshAll}
          safe={safe}
        />
      ) : null}
      <input value={configMapSearch} onChange={(e) => setConfigMapSearch(e.target.value)} placeholder="Search configmaps..." />
    </div>
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th><BulkSelectAll selection={selection} label="configmaps" /></th>
            <th><button className="sort-btn" onClick={() => toggleSort('configmaps', 'name')}>Name {sortMark('configmaps', 'name')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('configmaps', 'namespace')}>Namespace {sortMark('configmaps', 'namespace')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('configmaps', 'data_keys')}>Data Keys {sortMark('configmaps', 'data_keys')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('configmaps', 'age')}>Age {sortMark('configmaps', 'age')}</button></th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedConfigMaps.map((c) => (
            <tr key={namespacedResourceKey(c)} className={selection.selectedKeySet.has(namespacedResourceKey(c)) ? 'active-row' : ''}>
              <td><BulkRowSelect selection={selection} item={c} itemKey={namespacedResourceKey(c)} label={`${c.namespace}/${c.name}`} /></td>
              <td>{c.name}</td>
              <td>{c.namespace}</td>
              <td>{c.data_keys}</td>
              <td>{c.age}</td>
              <td className="actions-cell">
                <ActionMenu
                  actions={[
                    makeAction('Manifest', permissionInfo('configmaps', 'view', c.namespace), () => safe(() => openManifestTab(c.namespace, 'configmap', c.name))),
                    makeAction('Edit', permissionInfo('configmaps', 'edit', c.namespace), () => safe(() => openEditTab(c.namespace, 'configmap', c.name))),
                    makeAction('Delete', permissionInfo('configmaps', 'delete', c.namespace), () => safe(async () => {
                      await deleteResourceByRef('configmap', c.namespace, c.name);
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

export function CRDsPage({
  crdSearch,
  setCRDSearch,
  sortedCRDs,
  toggleSort,
  sortMark,
  makeAction,
  permissionInfo,
  safe,
  openManifestTab,
  primaryNamespace,
  allAllowed,
  openEditTab,
  deleteResourceByRef,
  refreshAll,
  selectedCRD,
  selectedCRDGroup,
  customResourceSearch,
  setCustomResourceSearch,
  sortedCustomResources,
  customResourceDefinition,
  selectCRDGroup
}) {
  const selection = useBulkSelection(sortedCRDs, clusterResourceKey);
  if (selectedCRD) {
    return (
      <CustomResourcesTable
        selectedCRD={selectedCRD}
        customResourceSearch={customResourceSearch}
        setCustomResourceSearch={setCustomResourceSearch}
        sortedCustomResources={sortedCustomResources}
        customResourceDefinition={customResourceDefinition}
        selectCRDGroup={selectCRDGroup}
        toggleSort={toggleSort}
        sortMark={sortMark}
        makeAction={makeAction}
        permissionInfo={permissionInfo}
        safe={safe}
        openManifestTab={openManifestTab}
        openEditTab={openEditTab}
        deleteResourceByRef={deleteResourceByRef}
        refreshAll={refreshAll}
      />
    );
  }
  return (
    <>
    <div className="toolbar fixed-toolbar">
      {selectedCRDGroup ? <button className="secondary" onClick={() => selectCRDGroup('')}>All CRDs</button> : null}
      {selectedCRDGroup ? <strong>{selectedCRDGroup}</strong> : null}
      <BulkSelectionCount selection={selection} />
      {selection.count > 0 ? (
        <BulkActionButton
          selection={selection}
          verb="Delete"
          className="danger"
          getPermission={() => permissionInfo('crds', 'delete')}
          runItem={(item) => deleteResourceByRef('crd', '', item.name)}
          refreshAll={refreshAll}
          safe={safe}
        />
      ) : null}
      <input value={crdSearch} onChange={(e) => setCRDSearch(e.target.value)} placeholder="Search CRDs..." />
    </div>
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th><BulkSelectAll selection={selection} label="CRDs" /></th>
            <th><button className="sort-btn" onClick={() => toggleSort('crds', 'name')}>Name {sortMark('crds', 'name')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('crds', 'group')}>Group {sortMark('crds', 'group')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('crds', 'kind')}>Kind {sortMark('crds', 'kind')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('crds', 'scope')}>Scope {sortMark('crds', 'scope')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('crds', 'versions')}>Versions {sortMark('crds', 'versions')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('crds', 'age')}>Age {sortMark('crds', 'age')}</button></th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedCRDs.map((crd) => (
            <tr key={crd.name} className={selection.selectedKeySet.has(clusterResourceKey(crd)) ? 'active-row' : ''}>
              <td><BulkRowSelect selection={selection} item={crd} itemKey={clusterResourceKey(crd)} label={crd.name} /></td>
              <td>{crd.name}</td>
              <td>{crd.group}</td>
              <td>{crd.kind}</td>
              <td>{crd.scope}</td>
              <td>{crd.versions || '-'}</td>
              <td>{crd.age}</td>
              <td className="actions-cell">
                <ActionMenu
                  actions={[
                    makeAction('Manifest', permissionInfo('crds', 'view'), () => safe(() => openManifestTab(primaryNamespace, 'crd', crd.name))),
                    makeAction('Edit', permissionInfo('crds', 'edit'), () => safe(() => openEditTab(primaryNamespace, 'crd', crd.name))),
                    makeAction('Delete', permissionInfo('crds', 'delete'), () => safe(async () => {
                      await deleteResourceByRef('crd', '', crd.name);
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

function CustomResourcesTable({
  selectedCRD,
  customResourceSearch,
  setCustomResourceSearch,
  sortedCustomResources,
  customResourceDefinition,
  selectCRDGroup,
  toggleSort,
  sortMark,
  makeAction,
  permissionInfo,
  safe,
  openManifestTab,
  openEditTab,
  deleteResourceByRef,
  refreshAll
}) {
  const resourceKind = `customresource:${selectedCRD.name}`;
  const selection = useBulkSelection(sortedCustomResources, customResourceKey);
  return (
    <>
      <div className="toolbar fixed-toolbar">
        <button className="secondary" onClick={() => selectCRDGroup('')}>All CRDs</button>
        <button className="secondary" onClick={() => selectCRDGroup(selectedCRD.group)}>{selectedCRD.group}</button>
        <strong>{selectedCRD.kind}</strong>
        <span className="small-hint">{selectedCRD.name}</span>
        <BulkSelectionCount selection={selection} />
        {selection.count > 0 ? (
          <BulkActionButton
            selection={selection}
            verb="Delete"
            className="danger"
            getPermission={(item) => permissionInfo('crds', 'delete', item.namespace)}
            runItem={(item) => deleteResourceByRef(resourceKind, item.namespace || '', item.name)}
            refreshAll={refreshAll}
            safe={safe}
          />
        ) : null}
        <input
          value={customResourceSearch}
          onChange={(e) => setCustomResourceSearch(e.target.value)}
          placeholder={`Search ${selectedCRD.kind} resources...`}
        />
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th><BulkSelectAll selection={selection} label={`${selectedCRD.kind} resources`} /></th>
              <th><button className="sort-btn" onClick={() => toggleSort('customresources', 'name')}>Name {sortMark('customresources', 'name')}</button></th>
              {customResourceDefinition?.namespaced ? (
                <th><button className="sort-btn" onClick={() => toggleSort('customresources', 'namespace')}>Namespace {sortMark('customresources', 'namespace')}</button></th>
              ) : null}
              <th><button className="sort-btn" onClick={() => toggleSort('customresources', 'api_version')}>API Version {sortMark('customresources', 'api_version')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('customresources', 'age')}>Age {sortMark('customresources', 'age')}</button></th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {sortedCustomResources.map((item) => (
              <tr key={customResourceKey(item)} className={selection.selectedKeySet.has(customResourceKey(item)) ? 'active-row' : ''}>
                <td><BulkRowSelect selection={selection} item={item} itemKey={customResourceKey(item)} label={`${item.namespace || 'cluster'}/${item.name}`} /></td>
                <td>{item.name}</td>
                {customResourceDefinition?.namespaced ? <td>{item.namespace}</td> : null}
                <td>{item.api_version}</td>
                <td>{item.age}</td>
                <td className="actions-cell">
                  <ActionMenu actions={[
                    makeAction('Manifest', permissionInfo('crds', 'view', item.namespace), () => safe(() => openManifestTab(item.namespace || '', resourceKind, item.name))),
                    makeAction('Edit', permissionInfo('crds', 'edit', item.namespace), () => safe(() => openEditTab(item.namespace || '', resourceKind, item.name))),
                    makeAction('Delete', permissionInfo('crds', 'delete', item.namespace), () => safe(async () => {
                      await deleteResourceByRef(resourceKind, item.namespace || '', item.name);
                      await refreshAll();
                    }))
                  ]} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {sortedCustomResources.length === 0 ? <div className="empty-state">No resources found.</div> : null}
      </div>
    </>
  );
}

export function SecretsPage({
  secretSearch,
  setSecretSearch,
  sortedSecrets,
  toggleSort,
  sortMark,
  makeAction,
  permissionInfo,
  currentUser,
  allAllowed,
  safe,
  openManifestTab,
  openEditTab,
  deleteResourceByRef,
  refreshAll
}) {
  const selection = useBulkSelection(sortedSecrets, namespacedResourceKey);
  return (
    <>
    <div className="toolbar fixed-toolbar">
      <BulkSelectionCount selection={selection} />
      {selection.count > 0 ? (
        <BulkActionButton
          selection={selection}
          verb="Delete"
          className="danger"
          getPermission={(item) => permissionInfo('secrets', 'delete', item.namespace)}
          runItem={(item) => deleteResourceByRef('secret', item.namespace, item.name)}
          refreshAll={refreshAll}
          safe={safe}
        />
      ) : null}
      <input value={secretSearch} onChange={(e) => setSecretSearch(e.target.value)} placeholder="Search secrets..." />
    </div>
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th><BulkSelectAll selection={selection} label="secrets" /></th>
            <th><button className="sort-btn" onClick={() => toggleSort('secrets', 'name')}>Name {sortMark('secrets', 'name')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('secrets', 'namespace')}>Namespace {sortMark('secrets', 'namespace')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('secrets', 'type')}>Type {sortMark('secrets', 'type')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('secrets', 'data_keys')}>Data Keys {sortMark('secrets', 'data_keys')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('secrets', 'age')}>Age {sortMark('secrets', 'age')}</button></th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedSecrets.map((s) => (
            <tr key={namespacedResourceKey(s)} className={selection.selectedKeySet.has(namespacedResourceKey(s)) ? 'active-row' : ''}>
              <td><BulkRowSelect selection={selection} item={s} itemKey={namespacedResourceKey(s)} label={`${s.namespace}/${s.name}`} /></td>
              <td>{s.name}</td>
              <td>{s.namespace}</td>
              <td>{s.type}</td>
              <td>{s.data_keys}</td>
              <td>{s.age}</td>
              <td className="actions-cell">
                <ActionMenu
                  actions={[
                    makeAction(
                      'Manifest',
                      permissionInfo('secrets', 'edit', s.namespace),
                      () => safe(() => openManifestTab(s.namespace, 'secret', s.name))
                    ),
                    makeAction('Edit', permissionInfo('secrets', 'edit', s.namespace), () => safe(() => openEditTab(s.namespace, 'secret', s.name))),
                    makeAction('Delete', permissionInfo('secrets', 'delete', s.namespace), () => safe(async () => {
                      await deleteResourceByRef('secret', s.namespace, s.name);
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

export function PVCsPage({
  pvcSearch,
  setPVCSearch,
  sortedPVCs,
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
  const selection = useBulkSelection(sortedPVCs, namespacedResourceKey);
  return (
    <>
    <div className="toolbar fixed-toolbar">
      <BulkSelectionCount selection={selection} />
      {selection.count > 0 ? (
        <BulkActionButton
          selection={selection}
          verb="Delete"
          className="danger"
          getPermission={(item) => permissionInfo('pvcs', 'delete', item.namespace)}
          runItem={(item) => deleteResourceByRef('pvc', item.namespace, item.name)}
          refreshAll={refreshAll}
          safe={safe}
        />
      ) : null}
      <input value={pvcSearch} onChange={(e) => setPVCSearch(e.target.value)} placeholder="Search PVCs..." />
    </div>
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th><BulkSelectAll selection={selection} label="PVCs" /></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvcs', 'name')}>Name {sortMark('pvcs', 'name')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvcs', 'namespace')}>Namespace {sortMark('pvcs', 'namespace')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvcs', 'status')}>Status {sortMark('pvcs', 'status')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvcs', 'volume')}>Volume {sortMark('pvcs', 'volume')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvcs', 'capacity')}>Capacity {sortMark('pvcs', 'capacity')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvcs', 'used_bytes')}>Usage {sortMark('pvcs', 'used_bytes')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvcs', 'storage_class')}>StorageClass {sortMark('pvcs', 'storage_class')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvcs', 'age')}>Age {sortMark('pvcs', 'age')}</button></th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedPVCs.map((p) => (
            <tr key={namespacedResourceKey(p)} className={selection.selectedKeySet.has(namespacedResourceKey(p)) ? 'active-row' : ''}>
              <td><BulkRowSelect selection={selection} item={p} itemKey={namespacedResourceKey(p)} label={`${p.namespace}/${p.name}`} /></td>
              <td>{p.name}</td>
              <td>{p.namespace}</td>
              <td>{p.status}</td>
              <td>{p.volume || '-'}</td>
              <td>{p.capacity}</td>
              <td>
                <div className="metric-cell">
                  <span>{p.usage || '-'}</span>
                  {!p.metrics_available ? (
                    <span className="metric-warning" title="kubelet volume stats are unavailable">!</span>
                  ) : null}
                </div>
              </td>
              <td>{p.storage_class}</td>
              <td>{p.age}</td>
              <td className="actions-cell">
                <ActionMenu
                  actions={[
                    makeAction('Manifest', permissionInfo('pvcs', 'view', p.namespace), () => safe(() => openManifestTab(p.namespace, 'pvc', p.name))),
                    makeAction('Edit', permissionInfo('pvcs', 'edit', p.namespace), () => safe(() => openEditTab(p.namespace, 'pvc', p.name))),
                    makeAction('Delete', permissionInfo('pvcs', 'delete', p.namespace), () => safe(async () => {
                      await deleteResourceByRef('pvc', p.namespace, p.name);
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

export function PVsPage({
  pvSearch,
  setPVSearch,
  sortedPVs,
  toggleSort,
  sortMark,
  makeAction,
  permissionInfo,
  safe,
  openManifestTab,
  primaryNamespace,
  allAllowed,
  openEditTab,
  deleteResourceByRef,
  refreshAll
}) {
  const selection = useBulkSelection(sortedPVs, clusterResourceKey);
  return (
    <>
    <div className="toolbar fixed-toolbar">
      <BulkSelectionCount selection={selection} />
      {selection.count > 0 ? (
        <BulkActionButton
          selection={selection}
          verb="Delete"
          className="danger"
          getPermission={() => permissionInfo('pvs', 'delete')}
          runItem={(item) => deleteResourceByRef('pv', '', item.name)}
          refreshAll={refreshAll}
          safe={safe}
        />
      ) : null}
      <input value={pvSearch} onChange={(e) => setPVSearch(e.target.value)} placeholder="Search PVs..." />
    </div>
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th><BulkSelectAll selection={selection} label="PVs" /></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvs', 'name')}>Name {sortMark('pvs', 'name')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvs', 'status')}>Status {sortMark('pvs', 'status')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvs', 'claim')}>Claim {sortMark('pvs', 'claim')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvs', 'capacity')}>Capacity {sortMark('pvs', 'capacity')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvs', 'used_bytes')}>Usage {sortMark('pvs', 'used_bytes')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvs', 'storage_class')}>StorageClass {sortMark('pvs', 'storage_class')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('pvs', 'age')}>Age {sortMark('pvs', 'age')}</button></th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedPVs.map((p) => (
            <tr key={p.name} className={selection.selectedKeySet.has(clusterResourceKey(p)) ? 'active-row' : ''}>
              <td><BulkRowSelect selection={selection} item={p} itemKey={clusterResourceKey(p)} label={p.name} /></td>
              <td>{p.name}</td>
              <td>{p.status}</td>
              <td>{p.claim}</td>
              <td>{p.capacity}</td>
              <td>
                <div className="metric-cell">
                  <span>{p.usage || '-'}</span>
                  {!p.metrics_available ? (
                    <span className="metric-warning" title="kubelet volume stats are unavailable">!</span>
                  ) : null}
                </div>
              </td>
              <td>{p.storage_class}</td>
              <td>{p.age}</td>
              <td className="actions-cell">
                <ActionMenu
                  actions={[
                    makeAction('Manifest', permissionInfo('pvs', 'view'), () => safe(() => openManifestTab(primaryNamespace, 'pv', p.name))),
                    makeAction('Edit', permissionInfo('pvs', 'edit'), () => safe(() => openEditTab(primaryNamespace, 'pv', p.name))),
                    makeAction('Delete', permissionInfo('pvs', 'delete'), () => safe(async () => {
                      await deleteResourceByRef('pv', '', p.name);
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

export function StorageClassesPage({
  sortedStorageClasses,
  toggleSort,
  sortMark,
  makeAction,
  permissionInfo,
  safe,
  openManifestTab,
  primaryNamespace,
  allAllowed,
  openEditTab,
  deleteResourceByRef,
  refreshAll
}) {
  const selection = useBulkSelection(sortedStorageClasses, clusterResourceKey);
  return (
    <>
    <div className="toolbar fixed-toolbar">
      <BulkSelectionCount selection={selection} />
      {selection.count > 0 ? (
        <BulkActionButton
          selection={selection}
          verb="Delete"
          className="danger"
          getPermission={() => permissionInfo('storageclasses', 'delete')}
          runItem={(item) => deleteResourceByRef('storageclass', '', item.name)}
          refreshAll={refreshAll}
          safe={safe}
        />
      ) : null}
    </div>
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th><BulkSelectAll selection={selection} label="storage classes" /></th>
            <th><button className="sort-btn" onClick={() => toggleSort('storageclasses', 'name')}>Name {sortMark('storageclasses', 'name')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('storageclasses', 'provisioner')}>Provisioner {sortMark('storageclasses', 'provisioner')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('storageclasses', 'reclaim_policy')}>Reclaim {sortMark('storageclasses', 'reclaim_policy')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('storageclasses', 'volume_binding_mode')}>Binding Mode {sortMark('storageclasses', 'volume_binding_mode')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('storageclasses', 'default_class')}>Default {sortMark('storageclasses', 'default_class')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('storageclasses', 'age')}>Age {sortMark('storageclasses', 'age')}</button></th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedStorageClasses.map((sc) => (
            <tr key={sc.name} className={selection.selectedKeySet.has(clusterResourceKey(sc)) ? 'active-row' : ''}>
              <td><BulkRowSelect selection={selection} item={sc} itemKey={clusterResourceKey(sc)} label={sc.name} /></td>
              <td>{sc.name}</td>
              <td>{sc.provisioner}</td>
              <td>{sc.reclaim_policy}</td>
              <td>{sc.volume_binding_mode}</td>
              <td>{sc.default_class ? 'yes' : 'no'}</td>
              <td>{sc.age}</td>
              <td className="actions-cell">
                <ActionMenu
                  actions={[
                    makeAction('Manifest', permissionInfo('storageclasses', 'view'), () => safe(() => openManifestTab(primaryNamespace, 'storageclass', sc.name))),
                    makeAction('Edit', permissionInfo('storageclasses', 'edit'), () => safe(() => openEditTab(primaryNamespace, 'storageclass', sc.name))),
                    makeAction('Delete', permissionInfo('storageclasses', 'delete'), () => safe(async () => {
                      await deleteResourceByRef('storageclass', '', sc.name);
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
