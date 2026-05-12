import React from 'react';
import { X } from 'lucide-react';
import ActionMenu from './ActionMenu.jsx';
import LogViewer from './LogViewer.jsx';
import YamlViewer from './YamlViewer.jsx';

export default function BottomDock({
  showBottomDock,
  startDockResize,
  bottomTabs,
  activeBottomTabId,
  setActiveBottomTabId,
  closeTab,
  activeBottomTab,
  upsertTab,
  scheduleLogsScrollToBottom,
  refreshLogTab,
  handleLogsScroll,
  logsOutputRef,
  logsEndRef,
  applyEditTab,
  permissionInfo,
  primaryNamespace,
  openNodePodsTab,
  makeAction,
  safe,
  openManifestTab,
  openPodLogsTab,
  evictPodByRef,
  setStatus,
  refreshAll,
  isDegradedReady,
  openPodExecTab,
  deletePodByRef,
  selectedPod,
  setSelectedPod,
  execTerminalHostRef
}) {
  if (!showBottomDock) {
    return null;
  }

  return (
    <>
      <div
        className="dock-resizer"
        onMouseDown={startDockResize}
        role="separator"
        aria-orientation="horizontal"
        aria-label="Resize bottom panel"
      />

      <div className="dock">
        <div className="dock-tabbar">
          {bottomTabs.map((tab) => (
            <div key={tab.id} className={`dock-tab ${tab.id === activeBottomTabId ? 'active' : ''}`}>
              <button
                className="dock-tab-main"
                onClick={() => setActiveBottomTabId(tab.id)}
                title={tab.title}
                aria-label={tab.title}
              >
                {tab.title}
              </button>
              <button className="dock-tab-close" onClick={() => closeTab(tab.id)} title={`Close ${tab.title}`} aria-label={`Close ${tab.title}`}>
                <X size={13} strokeWidth={1.8} aria-hidden="true" />
              </button>
            </div>
          ))}
        </div>

        <div className="dock-content">
          {activeBottomTab?.loading && <div className="dock-loading">Loading...</div>}
          {activeBottomTab?.error && <div className="dock-error">{activeBottomTab.error}</div>}

          {!activeBottomTab?.loading && !activeBottomTab?.error && activeBottomTab?.type === 'manifest' && (
            <YamlViewer text={activeBottomTab?.content || ''} />
          )}

          {!activeBottomTab?.loading && !activeBottomTab?.error && activeBottomTab?.type === 'logs' && (
            <div className="logs-pane">
              <div className="toolbar fixed-toolbar logs-toolbar">
                <input
                  value={activeBottomTab.search || ''}
                  onChange={(e) => upsertTab({ id: activeBottomTab.id, search: e.target.value }, false)}
                  placeholder="Search logs..."
                />
                <label className="toggle-row">
                  <input
                    type="checkbox"
                    checked={Boolean(activeBottomTab.showWarnings)}
                    onChange={(e) => upsertTab({ id: activeBottomTab.id, showWarnings: e.target.checked }, false)}
                  />
                  <span>Warnings</span>
                </label>
                <label className="toggle-row">
                  <input
                    type="checkbox"
                    checked={Boolean(activeBottomTab.showErrors)}
                    onChange={(e) => upsertTab({ id: activeBottomTab.id, showErrors: e.target.checked }, false)}
                  />
                  <span>Errors</span>
                </label>
                <label className="toggle-row">
                  <input
                    type="checkbox"
                    checked={Boolean(activeBottomTab.follow)}
                    onChange={(e) => {
                      if (e.target.checked) {
                        scheduleLogsScrollToBottom();
                      }
                      upsertTab({ id: activeBottomTab.id, follow: e.target.checked }, false);
                    }}
                  />
                  <span>Follow tail</span>
                </label>
                <button onClick={() => safe(() => refreshLogTab(activeBottomTab.id, false, { forceScrollToBottom: true }))}>Refresh</button>
              </div>
              <div className="logs-output-wrap" ref={logsOutputRef} onScroll={handleLogsScroll}>
                <LogViewer
                  text={activeBottomTab?.content || ''}
                  search={activeBottomTab.search || ''}
                  showWarnings={Boolean(activeBottomTab.showWarnings)}
                  showErrors={Boolean(activeBottomTab.showErrors)}
                  className="mono-block logs-output"
                />
                <div ref={logsEndRef} />
              </div>
              {activeBottomTab.loadingOlder ? <div className="small-hint">Loading older logs...</div> : null}
              {activeBottomTab.canLoadOlder === false ? <div className="small-hint">Beginning of log reached.</div> : null}
            </div>
          )}

          {!activeBottomTab?.loading && !activeBottomTab?.error && activeBottomTab?.type === 'edit' && (
            <div className="edit-pane">
              <div className="toolbar fixed-toolbar">
                <button
                  className="warn"
                  onClick={() => safe(() => applyEditTab(activeBottomTab.id, true))}
                  disabled={!permissionInfo('apply', 'edit', activeBottomTab?.namespace || primaryNamespace).allowed}
                  title={permissionInfo('apply', 'edit', activeBottomTab?.namespace || primaryNamespace).reason}
                >
                  Dry-run
                </button>
                <button
                  onClick={() => safe(() => applyEditTab(activeBottomTab.id, false))}
                  disabled={!permissionInfo('apply', 'edit', activeBottomTab?.namespace || primaryNamespace).allowed}
                  title={permissionInfo('apply', 'edit', activeBottomTab?.namespace || primaryNamespace).reason}
                >
                  Apply
                </button>
              </div>
              <textarea
                className="code-textarea"
                rows={14}
                value={activeBottomTab.content || ''}
                onChange={(e) => {
                  upsertTab({ id: activeBottomTab.id, content: e.target.value }, false);
                }}
              />
            </div>
          )}

          {!activeBottomTab?.loading && !activeBottomTab?.error && activeBottomTab?.type === 'node-pods' && (
            <div className="edit-pane">
              <div className="toolbar fixed-toolbar">
                <button onClick={() => safe(() => openNodePodsTab(activeBottomTab.nodeName))}>Refresh</button>
                <span className="small-hint">
                  {activeBottomTab.items?.length || 0} pods on {activeBottomTab.nodeName}
                </span>
              </div>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Pod</th>
                      <th>Status</th>
                      <th>Ready</th>
                      <th>Restarts</th>
                      <th>Age</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(activeBottomTab.items || []).map((pod) => (
                      <tr key={`${pod.namespace}/${pod.name}`}>
                        <td>{`${pod.namespace}/${pod.name}`}</td>
                        <td>{pod.phase}</td>
                        <td>{pod.ready}</td>
                        <td>{pod.restarts}</td>
                        <td>{pod.age}</td>
                        <td className="actions-cell">
                          <ActionMenu
                            actions={[
                              makeAction('Manifest', permissionInfo('pods', 'view', pod.namespace), () => safe(() => openManifestTab(pod.namespace, 'pod', pod.name))),
                              makeAction('Logs', permissionInfo('pods', 'view', pod.namespace), () => safe(() => openPodLogsTab(pod.namespace, pod.name))),
                              makeAction(
                                'Evict',
                                permissionInfo('pods', 'edit', pod.namespace),
                                async () => {
                                  await evictPodByRef(pod.namespace, pod.name);
                                  setStatus(`Eviction requested for ${pod.namespace}/${pod.name}`);
                                  await openNodePodsTab(activeBottomTab.nodeName);
                                  await refreshAll();
                                }
                              ),
                              makeAction(
                                'Exec',
                                {
                                  allowed:
                                    permissionInfo('pods', 'view', pod.namespace).allowed &&
                                    permissionInfo('exec', 'edit', pod.namespace).allowed &&
                                    String(pod.phase || '').toLowerCase() === 'running' &&
                                    !isDegradedReady(pod.ready),
                                  reason:
                                    !permissionInfo('pods', 'view', pod.namespace).allowed
                                      ? permissionInfo('pods', 'view', pod.namespace).reason
                                      : !permissionInfo('exec', 'edit', pod.namespace).allowed
                                        ? permissionInfo('exec', 'edit', pod.namespace).reason
                                        : 'Exec is only available for ready running pods'
                                },
                                () => safe(() => openPodExecTab(pod.namespace, pod.name))
                              ),
                              makeAction(
                                'Delete',
                                permissionInfo('pods', 'delete', pod.namespace),
                                async () => {
                                  await deletePodByRef(pod.namespace, pod.name);
                                  await openNodePodsTab(activeBottomTab.nodeName);
                                  if (selectedPod?.namespace === pod.namespace && selectedPod?.name === pod.name) {
                                    setSelectedPod(null);
                                  }
                                }
                              )
                            ]}
                          />
                        </td>
                      </tr>
                    ))}
                    {(!activeBottomTab.items || activeBottomTab.items.length === 0) && (
                      <tr>
                        <td colSpan="6" className="small-hint">No pods found for this node in selected namespaces.</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {!activeBottomTab?.loading && activeBottomTab?.type === 'exec' && (
            <div className="exec-pane">
              {activeBottomTab?.error ? <div className="dock-error">{activeBottomTab.error}</div> : null}
              <div className="exec-status">
                {activeBottomTab.connected ? 'Connected' : 'Disconnected'} · {activeBottomTab.connected ? 'Use Tab / arrows / Ctrl+C directly in terminal' : 'Terminal is read-only until exec shell starts successfully'}
              </div>
              <div className={`exec-terminal ${activeBottomTab.connected ? '' : 'is-readonly'}`.trim()} ref={execTerminalHostRef} />
            </div>
          )}
        </div>
      </div>
    </>
  );
}
