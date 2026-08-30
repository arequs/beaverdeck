import React from 'react';
import { BellRing } from 'lucide-react';
import ActionMenu from './ActionMenu.jsx';
import { BulkActionButton, BulkRowSelect, BulkSelectAll, BulkSelectionCount } from './BulkSelection.jsx';
import useBulkSelection from '../hooks/useBulkSelection.js';

function workloadKey(workload) {
  return `${workload.namespace}/${workload.kind}/${workload.name}`;
}

export default function WorkloadsPage({
  workloadSearch,
  setWorkloadSearch,
  sortedWorkloads,
  toggleSort,
  sortMark,
  isDegradedReady,
  showWarningPopover,
  scheduleWarningPopoverHide,
  makeAction,
  permissionInfo,
  allAllowed,
  safe,
  openManifestTab,
  openEditTab,
  openWorkloadLogsTab,
  openScaleModal,
  restartDeploymentByRef,
  deleteResourceByRef,
  refreshAll
}) {
  const selection = useBulkSelection(sortedWorkloads, workloadKey);
  const selectionMode = selection.count > 0;

  return (
    <>
      <div className="toolbar fixed-toolbar">
        <BulkSelectionCount selection={selection} />
        {selectionMode ? (
          <BulkActionButton
            selection={selection}
            verb="Restart"
            className="warn"
            getPermission={(workload) => allAllowed(
              permissionInfo('workloads', 'edit', workload.namespace),
              workload.kind === 'Deployment'
                ? { allowed: true, reason: '' }
                : { allowed: false, reason: 'Bulk restart supports Deployments only' }
            )}
            runItem={(workload) => restartDeploymentByRef(workload.namespace, workload.name)}
            refreshAll={refreshAll}
            safe={safe}
          />
        ) : null}
        {selectionMode ? (
          <BulkActionButton
            selection={selection}
            verb="Delete"
            className="danger"
            getPermission={(workload) => permissionInfo('workloads', 'delete', workload.namespace)}
            runItem={(workload) => deleteResourceByRef(workload.kind, workload.namespace, workload.name)}
            refreshAll={refreshAll}
            safe={safe}
          />
        ) : null}
        <input value={workloadSearch} onChange={(e) => setWorkloadSearch(e.target.value)} placeholder="Search workloads..." />
      </div>
      <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th><BulkSelectAll selection={selection} label="workloads" /></th>
            <th><button className="sort-btn" onClick={() => toggleSort('workloads', 'kind')}>Kind {sortMark('workloads', 'kind')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('workloads', 'name')}>Name {sortMark('workloads', 'name')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('workloads', 'namespace')}>Namespace {sortMark('workloads', 'namespace')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('workloads', 'ready')}>Ready {sortMark('workloads', 'ready')}</button></th>
            <th><button className="sort-btn" onClick={() => toggleSort('workloads', 'age')}>Age {sortMark('workloads', 'age')}</button></th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedWorkloads.map((w) => (
            <tr key={workloadKey(w)} className={selection.selectedKeySet.has(workloadKey(w)) ? 'active-row' : ''}>
              <td><BulkRowSelect selection={selection} item={w} itemKey={workloadKey(w)} label={`${w.namespace}/${w.name}`} /></td>
              <td>{w.kind}</td>
              <td>{w.name}</td>
              <td>{w.namespace}</td>
              <td>
                <div className="ready-cell">
                  <span>{w.ready}</span>
                  {isDegradedReady(w.ready) ? (
                    <button
                      className="warning-indicator"
                      title="View workload events"
                      onMouseEnter={(e) => {
                        void showWarningPopover(e, { type: 'workload', key: `workload:${w.namespace}:${w.kind}:${w.name}`, item: w });
                      }}
                      onMouseLeave={() => scheduleWarningPopoverHide(`workload:${w.namespace}:${w.kind}:${w.name}`)}
                      onClick={(e) => e.stopPropagation()}
                    >
                      <BellRing size={13} strokeWidth={2} aria-hidden="true" />
                    </button>
                  ) : null}
                </div>
              </td>
              <td>{w.age}</td>
              <td className="actions-cell">
                <ActionMenu
                  actions={[
                    makeAction('Manifest', permissionInfo('workloads', 'view', w.namespace), () => safe(() => openManifestTab(w.namespace, w.kind, w.name))),
                    makeAction('Edit', permissionInfo('workloads', 'edit', w.namespace), () => safe(() => openEditTab(w.namespace, w.kind, w.name))),
                    makeAction('Logs', permissionInfo('workloads', 'view', w.namespace), () => safe(() => openWorkloadLogsTab(w.namespace, w.kind, w.name))),
                    makeAction(
                      'Scale',
                      allAllowed(
                        permissionInfo('workloads', 'edit', w.namespace),
                        ['Deployment', 'StatefulSet'].includes(String(w.kind))
                          ? { allowed: true, reason: '' }
                          : { allowed: false, reason: 'Scale is currently available only for Deployments and StatefulSets' }
                      ),
                      () => openScaleModal(w)
                    ),
                    makeAction(
                      'Restart',
                      allAllowed(
                        permissionInfo('workloads', 'edit', w.namespace),
                        String(w.kind) === 'Deployment'
                          ? { allowed: true, reason: '' }
                          : { allowed: false, reason: 'Restart is currently available only for Deployments' }
                      ),
                      () => safe(async () => {
                        await restartDeploymentByRef(w.namespace, w.name);
                        await refreshAll();
                      })
                    ),
                    makeAction(
                      'Delete',
                      permissionInfo('workloads', 'delete', w.namespace),
                      () => safe(async () => {
                        await deleteResourceByRef(w.kind, w.namespace, w.name);
                        await refreshAll();
                      })
                    )
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
