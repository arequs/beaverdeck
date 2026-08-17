import React from 'react';
import { BellRing } from 'lucide-react';
import ActionMenu from './ActionMenu.jsx';

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
  setDeploymentName,
  setDeploymentNamespace,
  restartDeployment,
  deleteResourceByRef,
  refreshAll
}) {
  return (
    <>
      <div className="toolbar fixed-toolbar">
        <input value={workloadSearch} onChange={(e) => setWorkloadSearch(e.target.value)} placeholder="Search workloads..." />
      </div>
      <div className="table-wrap">
      <table>
        <thead>
          <tr>
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
            <tr key={`${w.namespace}/${w.kind}/${w.name}`}>
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
                        setDeploymentName(w.name);
                        setDeploymentNamespace(w.namespace);
                        await restartDeployment();
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
