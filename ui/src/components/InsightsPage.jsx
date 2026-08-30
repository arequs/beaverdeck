import React from 'react';
import { Info } from 'lucide-react';
import { getInsightDocumentationUrl } from '../lib/insightDocumentation.js';

function InsightHelpButton({ checkType }) {
  const documentationUrl = getInsightDocumentationUrl(checkType);
  if (!documentationUrl) return null;

  return (
    <div className="insight-help-row">
      <span>Learn why:</span>
      <button
        type="button"
        className="insight-help-button"
        title="Why is this an issue?"
        aria-label="Why is this an issue?"
        onClick={() => window.open(documentationUrl, '_blank', 'noopener,noreferrer')}
      >
        <Info size={14} strokeWidth={1.8} aria-hidden="true" />
      </button>
    </div>
  );
}

const LOG_RESOURCE_KINDS = ['Pod', 'Deployment', 'StatefulSet', 'DaemonSet'];

function insightTarget(alert) {
  if (alert.resource_kind && alert.resource_name) {
    const name = alert.namespace
      ? `${alert.namespace}/${alert.resource_name}`
      : alert.resource_name;
    return `${alert.resource_kind}: ${name}`;
  }
  if (alert.namespace) return `Namespace: ${alert.namespace}`;
  if (alert.node) return `Node: ${alert.node}`;
  return alert.title || 'Cluster-wide result';
}

function groupCountLabel(group) {
  const parts = [];
  if (group.alertCount > 0) parts.push(`${group.alertCount} alert${group.alertCount === 1 ? '' : 's'}`);
  if (group.passingCount > 0) parts.push(`${group.passingCount} passing`);
  if (group.suppressedCount > 0) parts.push(`${group.suppressedCount} suppressed`);
  return parts.join(' · ');
}

export default function InsightsPage({
  categoryLabel,
  showAllInsightChecks,
  setShowAllInsightChecks,
  showSuppressedInsights,
  setShowSuppressedInsights,
  refreshInsights,
  insightsSummary,
  selectAllInsightTypes,
  clearInsightTypes,
  availableInsightTypes,
  selectedInsightTypes,
  toggleInsightType,
  sortedInsights,
  groupedInsights,
  openInsightResource,
  openInsightLogs,
  safe,
  setInsightSuppressed
}) {
  return (
    <>
      <div className="toolbar fixed-toolbar">
        <div className="insight-category-label">
          <span className="small-label">Insights</span>
          <strong>{categoryLabel}</strong>
        </div>
        <label className="toggle-row">
          <input
            type="checkbox"
            checked={showAllInsightChecks}
            onChange={(e) => setShowAllInsightChecks(e.target.checked)}
          />
          <span>Show all checks</span>
        </label>
        <label className="toggle-row">
          <input
            type="checkbox"
            checked={showSuppressedInsights}
            onChange={(e) => setShowSuppressedInsights(e.target.checked)}
          />
          <span>Show suppressed alerts</span>
        </label>
        <button onClick={() => safe(refreshInsights)}>Refresh insights</button>
        <div className="insights-summary">
          <span>Checks: {insightsSummary.total || 0}</span>
          <span>Alerts: {insightsSummary.alerts || 0}</span>
          <span>Active: {insightsSummary.active || 0}</span>
          <span>Passing: {insightsSummary.passing || 0}</span>
          <span>Suppressed: {insightsSummary.suppressed || 0}</span>
        </div>
      </div>

      <div className="insight-filters">
        <div className="insight-filters-head">
          <div className="small-label">Check Types</div>
          <div className="insight-filters-actions">
            <button onClick={selectAllInsightTypes}>All</button>
            <button onClick={clearInsightTypes}>None</button>
          </div>
        </div>
        <div className="insight-filter-list">
          {availableInsightTypes.map((item) => (
            <label key={item.value} className="insight-filter-item">
              <input
                type="checkbox"
                checked={selectedInsightTypes.includes(item.value)}
                onChange={() => toggleInsightType(item.value)}
              />
              <span>{item.label}</span>
            </label>
          ))}
        </div>
      </div>

      {selectedInsightTypes.length === 0 && (
        <div className="empty-state">
          <strong>No check types selected</strong>
          <span>Select one or more check types to see results.</span>
        </div>
      )}

      {selectedInsightTypes.length > 0 && sortedInsights.length === 0 && !showAllInsightChecks && !showSuppressedInsights && (
        <div className="empty-state">
          <strong>All is good</strong>
          <span>No active alerts for the selected check types and namespaces.</span>
        </div>
      )}

      {selectedInsightTypes.length > 0 && sortedInsights.length === 0 && (showSuppressedInsights || showAllInsightChecks) && (
        <div className="empty-state">
          <strong>Nothing to show</strong>
          <span>No checks match the current filters.</span>
        </div>
      )}

      {groupedInsights.length > 0 && (
        <div className="insights-grid">
          {groupedInsights.map((group) => (
            <section key={group.key} className="insight-dashboard">
              <article
                className={`insight-card insight-group-card severity-${group.severity} status-${group.alertCount > 0 ? 'alert' : 'ok'}`}
              >
                <div className="insight-card-head">
                  <div>
                    <div className="small-label">{group.category}</div>
                    <h2>{group.label}</h2>
                  </div>
                  <div className="insight-group-status">
                    <span className={`severity-badge severity-${group.severity}`}>
                      {group.alertCount > 0 ? group.severity : 'ok'}
                    </span>
                    <span className="insight-dashboard-count">{groupCountLabel(group)}</span>
                  </div>
                </div>
                <p className="insight-summary">
                  {(group.items.find((item) => item.status === 'alert') || group.items[0])?.summary}
                </p>
                <div className="insight-group-results">
                  {group.items.map((alert) => (
                    <div
                      key={alert.key}
                      className={`insight-result-row status-${String(alert.status || 'alert').toLowerCase()}`}
                    >
                      <div className="insight-result-main">
                        <div className="insight-result-head">
                          <strong>{insightTarget(alert)}</strong>
                          {(String(alert.severity || 'warning').toLowerCase() !== group.severity) ? (
                            <span className={`severity-badge severity-${String(alert.severity || 'warning').toLowerCase()}`}>
                              {alert.status === 'alert' ? alert.severity : 'ok'}
                            </span>
                          ) : null}
                        </div>
                        {(alert.node && alert.resource_kind !== 'Node') || alert.suppressed ? (
                          <div className="insight-meta">
                            {alert.node && alert.resource_kind !== 'Node' ? <span>Node: {alert.node}</span> : null}
                            {alert.suppressed ? <span>Suppressed</span> : null}
                          </div>
                        ) : null}
                        {Array.isArray(alert.details) && alert.details.length > 0 ? (
                          <ul className="insight-details">
                            {alert.details.map((detail) => (
                              <li key={detail}>{detail}</li>
                            ))}
                          </ul>
                        ) : null}
                      </div>
                      {(alert.resource_kind && alert.resource_name) || alert.status === 'alert' ? (
                        <div className="insight-actions insight-result-actions">
                          {alert.resource_kind && alert.resource_name ? (
                            <button onClick={() => openInsightResource(alert)}>
                              Open Resource
                            </button>
                          ) : null}
                          {LOG_RESOURCE_KINDS.includes(String(alert.resource_kind || '')) ? (
                            <button onClick={() => openInsightLogs(alert)}>
                              Open Logs
                            </button>
                          ) : null}
                          {alert.status === 'alert' ? (
                            <button onClick={() => safe(() => setInsightSuppressed(alert.key, !alert.suppressed))}>
                              {alert.suppressed ? 'Restore' : 'Ignore'}
                            </button>
                          ) : null}
                        </div>
                      ) : null}
                    </div>
                  ))}
                </div>
                <InsightHelpButton checkType={group.checkType} />
              </article>
            </section>
          ))}
        </div>
      )}
    </>
  );
}
