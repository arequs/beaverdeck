import React from 'react';

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
            <section key={group.label} className="insight-dashboard">
              <div className="insight-dashboard-head">
                <div>
                  <div className="small-label">Dashboard</div>
                  <h2>{group.label}</h2>
                  <div className="insight-dashboard-subtitle">{group.category}</div>
                </div>
                <span className="insight-dashboard-count">{group.items.length} checks</span>
              </div>
              <div className="insight-dashboard-cards">
                {group.items.map((alert) => (
                  <article
                    key={alert.key}
                    className={`insight-card severity-${String(alert.severity || 'warning').toLowerCase()} status-${String(alert.status || 'alert').toLowerCase()}`}
                  >
                    <div className="insight-card-head">
                      <div>
                        <div className="small-label">{alert.status === 'alert' ? alert.category : `${alert.category} Check`}</div>
                        <h3>{alert.title}</h3>
                      </div>
                      <div className="insight-actions">
                        <span className={`severity-badge severity-${String(alert.severity || 'warning').toLowerCase()}`}>
                          {alert.status === 'alert' ? alert.severity : 'ok'}
                        </span>
                        <button onClick={() => openInsightResource(alert)} disabled={!alert.resource_kind || !alert.resource_name}>
                          Open Resource
                        </button>
                        <button
                          onClick={() => openInsightLogs(alert)}
                          disabled={!['Pod', 'Deployment', 'StatefulSet', 'DaemonSet'].includes(String(alert.resource_kind || ''))}
                        >
                          Open Logs
                        </button>
                        {alert.status === 'alert' ? (
                          <button onClick={() => safe(() => setInsightSuppressed(alert.key, !alert.suppressed))}>
                            {alert.suppressed ? 'Restore' : 'Ignore'}
                          </button>
                        ) : null}
                      </div>
                    </div>
                    <p className="insight-summary">{alert.summary}</p>
                    <div className="insight-meta">
                      {alert.namespace ? <span>Namespace: {alert.namespace}</span> : null}
                      {alert.node ? <span>Node: {alert.node}</span> : null}
                      {alert.resource_kind && alert.resource_name ? <span>{alert.resource_kind}: {alert.resource_name}</span> : null}
                      {alert.suppressed ? <span>Suppressed</span> : null}
                    </div>
                    {Array.isArray(alert.details) && alert.details.length > 0 ? (
                      <ul className="insight-details">
                        {alert.details.map((detail) => (
                          <li key={detail}>{detail}</li>
                        ))}
                      </ul>
                    ) : null}
                  </article>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
    </>
  );
}
