import React from 'react';
import { formatByteValue, formatMilliValue } from '../lib/appUtils.js';

function formatTime(value) {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString();
}

function offsetLabel(seconds) {
  const value = Math.abs(Number(seconds || 0));
  if (value >= 60) return `T-${value / 60}m`;
  return `T-${value}s`;
}

function metricValue(point, field, formatter) {
  if (!point?.available) return 'Unavailable';
  return formatter(Number(point[field] || 0));
}

function usageValue(available, used, total, formatter) {
  if (!available) return `Unavailable / ${formatter(total)}`;
  const percentage = total > 0 ? ` (${Math.round((used / total) * 100)}%)` : '';
  return `${formatter(used)} / ${formatter(total)}${percentage}`;
}

export default function RestartDiagnosticModal({ open, loading, error, snapshot, onClose }) {
  if (!open) return null;
  const resources = snapshot?.container?.resources || {};
  const containerMetrics = snapshot?.container_metrics || [];
  const nodeMetricsByOffset = new Map((snapshot?.node_metrics || []).map((item) => [item.offset_seconds, item]));
  const nodeResources = snapshot?.node_resources || {};

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card restart-diagnostic-modal" onClick={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <div>
            <h3>Restart diagnostics</h3>
            {snapshot ? (
              <p className="modal-muted">
                {snapshot.workload?.kind}/{snapshot.workload?.name} · {snapshot.container?.name} · {formatTime(snapshot.incident_time)}
              </p>
            ) : null}
          </div>
          <button className="modal-close" type="button" aria-label="Close" onClick={onClose}>×</button>
        </div>

        <div className="restart-diagnostic-body">
          {loading ? <div className="diagnostic-state">Loading incident snapshot…</div> : null}
          {error ? <div className="diagnostic-state diagnostic-state-error">{error}</div> : null}
          {!loading && !error && snapshot ? (
            <div className="diagnostic-content">
            <section className="diagnostic-summary-grid">
              <div><span>Classification</span><strong>{snapshot.classification || 'termination'}</strong></div>
              <div><span>Reason</span><strong>{snapshot.reason || 'Not reported'}</strong></div>
              <div><span>Exit code</span><strong>{snapshot.exit_code ?? '-'}</strong></div>
              <div><span>Restart count</span><strong>{snapshot.pod?.restart_count ?? snapshot.container?.restart_count ?? '-'}</strong></div>
              <div><span>Pod</span><strong>{snapshot.pod?.namespace}/{snapshot.pod?.name}</strong></div>
              <div><span>Node</span><strong>{snapshot.pod?.node || '-'}</strong></div>
            </section>

            {snapshot.message ? <div className="diagnostic-message">{snapshot.message}</div> : null}

            <section>
              <h4>Metrics before incident</h4>
              <div className="table-wrap diagnostic-table-wrap">
                <table>
                  <thead><tr><th>Point</th><th>Container CPU</th><th>Container memory</th><th>Node CPU</th><th>Node memory</th><th>Sample</th></tr></thead>
                  <tbody>
                    {containerMetrics.map((point) => {
                      const nodePoint = nodeMetricsByOffset.get(point.offset_seconds);
                      return (
                        <tr key={point.offset_seconds}>
                          <td>{offsetLabel(point.offset_seconds)}</td>
                          <td>{metricValue(point, 'cpu_used_milli', formatMilliValue)}</td>
                          <td>{metricValue(point, 'memory_bytes', formatByteValue)}</td>
                          <td>{metricValue(nodePoint, 'cpu_used_milli', formatMilliValue)}</td>
                          <td>{metricValue(nodePoint, 'memory_bytes', formatByteValue)}</td>
                          <td>{(point.available || nodePoint?.available) ? `${formatTime(point.sampled_at || nodePoint?.sampled_at)} · ${point.source || nodePoint?.source || 'Kubernetes'}` : 'Unavailable'}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </section>

            <section>
              <h4>Node resources at incident</h4>
              <div className="diagnostic-resource-grid">
                <div><span>CPU usage / allocatable</span><strong>{usageValue(nodeResources.metrics_available, nodeResources.cpu_used_milli, nodeResources.cpu_allocatable_milli, formatMilliValue)}</strong></div>
                <div><span>Memory usage / allocatable</span><strong>{usageValue(nodeResources.metrics_available, nodeResources.memory_used_bytes, nodeResources.memory_allocatable_bytes, formatByteValue)}</strong></div>
                <div><span>CPU capacity</span><strong>{formatMilliValue(nodeResources.cpu_capacity_milli)}</strong></div>
                <div><span>Memory capacity</span><strong>{formatByteValue(nodeResources.memory_capacity_bytes)}</strong></div>
              </div>
            </section>

            <section>
              <h4>Container requests and limits</h4>
              <div className="diagnostic-resource-grid">
                <div><span>CPU request</span><strong>{formatMilliValue(resources.cpu_request_milli)}</strong></div>
                <div><span>CPU limit</span><strong>{formatMilliValue(resources.cpu_limit_milli)}</strong></div>
                <div><span>Memory request</span><strong>{formatByteValue(resources.memory_request_bytes)}</strong></div>
                <div><span>Memory limit</span><strong>{formatByteValue(resources.memory_limit_bytes)}</strong></div>
              </div>
            </section>

            <section>
              <h4>Pods on {snapshot.pod?.node || 'node'} near incident</h4>
              {(snapshot.node_pods || []).length ? (
                <div className="table-wrap diagnostic-table-wrap diagnostic-node-pods">
                  <table>
                    <thead><tr><th>Namespace</th><th>Pod</th><th>CPU</th><th>Memory</th></tr></thead>
                    <tbody>
                      {snapshot.node_pods.map((item) => (
                        <tr key={`${item.namespace}/${item.pod}`} className={item.affected ? 'diagnostic-affected-row' : ''}>
                          <td>{item.namespace}</td><td>{item.pod}</td><td>{formatMilliValue(item.cpu_used_milli)}</td><td>{formatByteValue(item.memory_used_bytes)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : <div className="diagnostic-state">Node pod metrics unavailable.</div>}
            </section>

            <section>
              <h4>Relevant Kubernetes events</h4>
              {(snapshot.events || []).length ? (
                <div className="diagnostic-events">
                  {snapshot.events.map((item, index) => (
                    <div className="diagnostic-event" key={`${item.time}-${item.reason}-${index}`}>
                      <time>{formatTime(item.time)}</time>
                      <strong>{item.reason || item.type}</strong>
                      <span>{item.object}</span>
                      <p>{item.message}</p>
                    </div>
                  ))}
                </div>
              ) : <div className="diagnostic-state">No relevant events captured.</div>}
            </section>

            <section>
              <h4>Persistent storage</h4>
              {(snapshot.persistent_storage || []).length ? (
                <div className="table-wrap diagnostic-table-wrap">
                  <table>
                    <thead><tr><th>Volume</th><th>PVC</th><th>PVC status</th><th>PV</th><th>PV status</th><th>Storage class</th><th>Requested</th><th>Capacity</th><th>Access</th></tr></thead>
                    <tbody>
                      {snapshot.persistent_storage.map((item) => (
                        <tr key={`${item.volume}/${item.claim_namespace}/${item.claim_name}`}>
                          <td>{item.volume}</td>
                          <td>{item.claim_namespace}/{item.claim_name}</td>
                          <td>{item.claim_status || item.error || '-'}</td>
                          <td>{item.volume_name || '-'}</td>
                          <td>{item.volume_status || '-'}</td>
                          <td>{item.storage_class || '-'}</td>
                          <td>{formatByteValue(item.requested_bytes)}</td>
                          <td>{formatByteValue(item.capacity_bytes)}</td>
                          <td>{(item.access_modes || []).join(', ') || item.volume_mode || '-'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : <div className="diagnostic-state">Not found.</div>}
            </section>

            <section>
              <h4>PV / PVC events</h4>
              {(snapshot.storage_events || []).length ? (
                <div className="diagnostic-events">
                  {snapshot.storage_events.map((item, index) => (
                    <div className="diagnostic-event" key={`${item.time}-${item.object}-${index}`}>
                      <time>{formatTime(item.time)}</time>
                      <strong>{item.reason || item.type}</strong>
                      <span>{item.object}</span>
                      <p>{item.message}</p>
                    </div>
                  ))}
                </div>
              ) : <div className="diagnostic-state">No PV/PVC events captured.</div>}
            </section>

            <section>
              <h4>Previous container logs</h4>
              <div className="diagnostic-logs">
                {(snapshot.previous_logs || []).map((item) => (
                  <details key={`${item.init ? 'init' : 'container'}-${item.container}`} open={item.container === snapshot.container?.name}>
                    <summary>{item.init ? 'Init container' : 'Container'}: {item.container}{item.truncated ? ' · truncated' : ''}</summary>
                    {item.available ? <pre>{item.text || '(empty log)'}</pre> : <div className="diagnostic-state">{item.error || 'Previous logs unavailable.'}</div>}
                  </details>
                ))}
                {(snapshot.previous_logs || []).length === 0 ? <div className="diagnostic-state">Previous logs unavailable.</div> : null}
              </div>
            </section>

            {(snapshot.warnings || []).length ? <div className="diagnostic-warnings">{snapshot.warnings.join(' · ')}</div> : null}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
