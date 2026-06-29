import React, { useEffect, useMemo, useRef } from 'react';
import { Logs, SquareTerminal } from 'lucide-react';
import ActionMenu from './ActionMenu.jsx';

function podRefKey(namespace, name) {
  return `${String(namespace || '').trim()}/${String(name || '').trim()}`;
}

export default function PodsPage({
  podSearch,
  setPodSearch,
  podStatusFilter,
  setPodStatusFilter,
  availablePodStatuses,
  podNameRegexError,
  podsAutoRefreshEnabled,
  setPodsAutoRefreshEnabled,
  podsAutoRefreshSeconds,
  setPodsAutoRefreshSeconds,
  sortedPods,
  selectedPodRefSet,
  selectedPodCount,
  togglePodRefSelection,
  setPodRefsSelection,
  selectedPodEvictPermission,
  selectedPodDeletePermission,
  toggleSort,
  sortMark,
  selectedPod,
  selectPod,
  isDegradedReady,
  showWarningPopover,
  scheduleWarningPopoverHide,
  makeAction,
  permissionInfo,
  safe,
  openManifestTab,
  openPodLogsTab,
  allAllowed,
  openPodExecTab,
  evictPodByRef,
  deletePodByRef,
  setSelectedPod,
  refreshAll,
  deleteSelectedPods,
  evictSelectedPods
}) {
  const selectAllRef = useRef(null);
  const visiblePodRefs = useMemo(
    () => sortedPods.map((pod) => podRefKey(pod.namespace, pod.name)),
    [sortedPods]
  );
  const allVisibleSelected = visiblePodRefs.length > 0 && visiblePodRefs.every((ref) => selectedPodRefSet.has(ref));
  const someVisibleSelected = visiblePodRefs.some((ref) => selectedPodRefSet.has(ref));

  useEffect(() => {
    if (!selectAllRef.current) {
      return;
    }
    selectAllRef.current.indeterminate = someVisibleSelected && !allVisibleSelected;
  }, [someVisibleSelected, allVisibleSelected]);

  const podSelectionMode = selectedPodCount > 0;
  const selectionDisabledCheck = podSelectionMode
    ? { allowed: false, reason: 'Actions are disabled while pods are selected' }
    : { allowed: true, reason: '' };

  return (
    <>
      <div className="toolbar fixed-toolbar">
        {podSelectionMode ? <span className="small-hint">{selectedPodCount} selected</span> : null}
        {podSelectionMode ? (
          <button
            type="button"
            className="warn"
            disabled={!selectedPodEvictPermission.allowed}
            title={selectedPodEvictPermission.reason}
            onClick={() => safe(evictSelectedPods)}
          >
            Evict
          </button>
        ) : null}
        {podSelectionMode ? (
          <button
            type="button"
            className="danger"
            disabled={!selectedPodDeletePermission.allowed}
            title={selectedPodDeletePermission.reason}
            onClick={() => safe(deleteSelectedPods)}
          >
            Delete
          </button>
        ) : null}
        <input value={podSearch} onChange={(e) => setPodSearch(e.target.value)} placeholder="Name regex..." />
        <select value={podStatusFilter} onChange={(e) => setPodStatusFilter(e.target.value)}>
          <option value="">All statuses</option>
          {availablePodStatuses.map((status) => (
            <option key={status} value={status}>{status}</option>
          ))}
        </select>
        <label className="toggle-row">
          <input
            type="checkbox"
            checked={podsAutoRefreshEnabled}
            onChange={(e) => setPodsAutoRefreshEnabled(e.target.checked)}
          />
          <span>Auto refresh</span>
        </label>
        <select
          value={String(podsAutoRefreshSeconds)}
          onChange={(e) => setPodsAutoRefreshSeconds(Number(e.target.value))}
          disabled={!podsAutoRefreshEnabled}
        >
          <option value="1">1s</option>
          <option value="5">5s</option>
          <option value="15">15s</option>
        </select>
        {podNameRegexError ? <span className="small-hint">Invalid regex: {podNameRegexError}</span> : null}
      </div>

      <div className="table-wrap pods-table-wrap">
        <table>
          <thead>
            <tr>
              <th>
                <input
                  ref={selectAllRef}
                  type="checkbox"
                  checked={allVisibleSelected}
                  disabled={sortedPods.length === 0}
                  onChange={(e) => setPodRefsSelection(sortedPods, e.target.checked)}
                  aria-label={allVisibleSelected ? 'Unselect all visible pods' : 'Select all visible pods'}
                />
              </th>
              <th><button className="sort-btn" onClick={() => toggleSort('pods', 'name')}>Name {sortMark('pods', 'name')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('pods', 'namespace')}>Namespace {sortMark('pods', 'namespace')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('pods', 'phase')}>Status {sortMark('pods', 'phase')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('pods', 'ready')}>Ready {sortMark('pods', 'ready')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('pods', 'restarts')}>Restarts {sortMark('pods', 'restarts')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('pods', 'node')}>Node {sortMark('pods', 'node')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('pods', 'age')}>Age {sortMark('pods', 'age')}</button></th>
              <th className="actions-head-cell"><span className="table-head-label">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            {sortedPods.map((p) => {
              const rowKey = podRefKey(p.namespace, p.name);
              const rowSelected = selectedPodRefSet.has(rowKey);
              const showPodWarning = isDegradedReady(p.ready) && String(p.phase || '').toLowerCase() !== 'succeeded';
              const logsPermission = permissionInfo('pods', 'view', p.namespace);
              const execPermission = allAllowed(
                permissionInfo('pods', 'view', p.namespace),
                permissionInfo('exec', 'edit', p.namespace),
                String(p.phase || '').toLowerCase() === 'running' && !isDegradedReady(p.ready)
                  ? { allowed: true, reason: '' }
                  : { allowed: false, reason: 'Exec is disabled for pods that are not fully running' }
              );

              return (
                <tr
                  key={rowKey}
                  className={
                    podSelectionMode
                      ? (rowSelected ? 'active-row' : '')
                      : (selectedPod?.namespace === p.namespace && selectedPod?.name === p.name ? 'active-row' : '')
                  }
                  onClick={() => {
                    if (!podSelectionMode) {
                      selectPod(p);
                    }
                  }}
                >
                  <td onClick={(e) => e.stopPropagation()}>
                    <input
                      type="checkbox"
                      checked={rowSelected}
                      onChange={() => togglePodRefSelection(p)}
                      aria-label={`Select ${p.namespace}/${p.name}`}
                    />
                  </td>
                  <td>{p.name}</td>
                  <td>{p.namespace}</td>
                  <td>{p.phase}</td>
                  <td>
                    <div className="ready-cell">
                      <span>{p.ready}</span>
                      {showPodWarning ? (
                        <button
                          type="button"
                          className="warning-indicator"
                          title="Pod is not fully ready"
                          onMouseEnter={(e) => {
                            void showWarningPopover(e, { type: 'pod', key: `pod:${p.namespace}:${p.name}`, item: p });
                          }}
                          onMouseLeave={() => scheduleWarningPopoverHide(`pod:${p.namespace}:${p.name}`)}
                          onClick={(e) => e.stopPropagation()}
                        >
                          !
                        </button>
                      ) : null}
                    </div>
                  </td>
                  <td>{p.restarts}</td>
                  <td>{p.node || '-'}</td>
                  <td>{p.age}</td>
                  <td className="actions-cell">
                    <div className="pod-inline-actions" onClick={(e) => e.stopPropagation()}>
                      <button
                        type="button"
                        className="pod-icon-button"
                        aria-label="Open pod logs"
                        disabled={!allAllowed(selectionDisabledCheck, logsPermission).allowed}
                        title={allAllowed(selectionDisabledCheck, logsPermission).allowed ? 'Logs' : allAllowed(selectionDisabledCheck, logsPermission).reason}
                        onClick={() => safe(() => openPodLogsTab(p.namespace, p.name, '', p.containers))}
                      >
                        <Logs className="pod-action-icon" size={14} strokeWidth={1.8} aria-hidden="true" />
                      </button>
                      <button
                        type="button"
                        className="pod-icon-button"
                        aria-label="Open pod exec"
                        disabled={!allAllowed(selectionDisabledCheck, execPermission).allowed}
                        title={allAllowed(selectionDisabledCheck, execPermission).allowed ? 'Exec' : allAllowed(selectionDisabledCheck, execPermission).reason}
                        onClick={() => safe(() => openPodExecTab(p.namespace, p.name, '', p.containers))}
                      >
                        <SquareTerminal className="pod-action-icon" size={14} strokeWidth={1.8} aria-hidden="true" />
                      </button>
                      <ActionMenu
                        actions={[
                          makeAction('Manifest', allAllowed(selectionDisabledCheck, permissionInfo('pods', 'view', p.namespace)), () => safe(() => openManifestTab(p.namespace, 'pod', p.name))),
                          makeAction(
                            'Evict',
                            allAllowed(selectionDisabledCheck, permissionInfo('pods', 'edit', p.namespace)),
                            () => safe(async () => {
                              await evictPodByRef(p.namespace, p.name);
                              await refreshAll();
                            })
                          ),
                          makeAction(
                            'Delete',
                            allAllowed(selectionDisabledCheck, permissionInfo('pods', 'delete', p.namespace)),
                            () => safe(async () => {
                              await deletePodByRef(p.namespace, p.name);
                              if (selectedPod?.namespace === p.namespace && selectedPod?.name === p.name) {
                                setSelectedPod(null);
                              }
                              await refreshAll();
                            })
                          )
                        ]}
                      />
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </>
  );
}
