import React from 'react';
import ActionMenu from './ActionMenu.jsx';

export default function NodesPage({
  availableNodeLabelKeys,
  applyNodeLabelKey,
  availableNodeLabelValues,
  parsedNodeLabelFilter,
  applyNodeLabelValue,
  nodeLabelFilter,
  setNodeLabelFilter,
  sortedNodes,
  toggleSort,
  sortMark,
  makeAction,
  allAllowed,
  selectedNamespaces,
  permissionInfo,
  primaryNamespace,
  safe,
  openNodePodsTab,
  openDrainModal,
  uncordonNodeAction,
  openManifestTab,
  openEditTab
}) {
  return (
    <>
      <div className="toolbar fixed-toolbar">
        <select
          value=""
          onChange={(e) => {
            applyNodeLabelKey(e.target.value);
            e.target.value = '';
          }}
        >
          <option value="">Select label key...</option>
          {availableNodeLabelKeys.map((key) => (
            <option key={key} value={key}>{key}</option>
          ))}
        </select>
        <select
          value={availableNodeLabelValues.includes(parsedNodeLabelFilter.value) ? parsedNodeLabelFilter.value : ''}
          onChange={(e) => applyNodeLabelValue(e.target.value)}
          disabled={!parsedNodeLabelFilter.key}
        >
          <option value="">Select label value...</option>
          {availableNodeLabelValues.map((value) => (
            <option key={value} value={value}>{value}</option>
          ))}
        </select>
        <input
          value={nodeLabelFilter}
          onChange={(e) => setNodeLabelFilter(e.target.value)}
          placeholder="Filter by label: key or key=value"
        />
        <button onClick={() => setNodeLabelFilter('')} disabled={!nodeLabelFilter.trim()}>
          Clear
        </button>
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th><button className="sort-btn" onClick={() => toggleSort('nodes', 'name')}>Name {sortMark('nodes', 'name')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('nodes', 'status')}>Status {sortMark('nodes', 'status')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('nodes', 'roles')}>Roles {sortMark('nodes', 'roles')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('nodes', 'pod_count')}>Pods {sortMark('nodes', 'pod_count')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('nodes', 'cpu_used_milli')}>CPU {sortMark('nodes', 'cpu_used_milli')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('nodes', 'memory_used_bytes')}>Memory {sortMark('nodes', 'memory_used_bytes')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('nodes', 'gpu_used_percent')}>GPU {sortMark('nodes', 'gpu_used_percent')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('nodes', 'age')}>Age {sortMark('nodes', 'age')}</button></th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {sortedNodes.map((n) => (
              <tr key={n.name}>
                <td>{n.name}</td>
                <td>{n.status}</td>
                <td>{n.roles}</td>
                <td>{n.pods || '-'}</td>
                <td>
                  <div className="metric-cell">
                    <span>{n.cpu || '-'}</span>
                    {!n.metrics_available ? (
                      <span className="metric-warning" title="Resource metrics are unavailable">!</span>
                    ) : null}
                  </div>
                </td>
                <td>
                  <div className="metric-cell">
                    <span>{n.memory || '-'}</span>
                    {!n.metrics_available ? (
                      <span className="metric-warning" title="Resource metrics are unavailable">!</span>
                    ) : null}
                  </div>
                </td>
                <td>
                  <div className="metric-cell">
                    <span>{n.gpu || '-'}</span>
                    {n.has_gpu && !n.gpu_metrics_available ? (
                      <span className="metric-warning" title="GPU metrics are unavailable">!</span>
                    ) : null}
                  </div>
                </td>
                <td>{n.age}</td>
                <td className="actions-cell">
                  <ActionMenu
                    actions={[
                      makeAction(
                        'View Pods',
                        allAllowed(
                          { allowed: selectedNamespaces.length > 0, reason: 'Select at least one namespace first' },
                          permissionInfo('pods', 'view', primaryNamespace || selectedNamespaces[0] || '')
                        ),
                        () => safe(() => openNodePodsTab(n.name))
                      ),
                      makeAction('Drain', permissionInfo('nodes', 'edit'), () => openDrainModal(n.name)),
                      makeAction('Uncordon', permissionInfo('nodes', 'edit'), () => safe(() => uncordonNodeAction(n.name))),
                      makeAction('Manifest', permissionInfo('nodes', 'view'), () => safe(() => openManifestTab(primaryNamespace, 'node', n.name))),
                      makeAction('Edit', permissionInfo('nodes', 'edit'), () => safe(() => openEditTab(primaryNamespace, 'node', n.name)))
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
