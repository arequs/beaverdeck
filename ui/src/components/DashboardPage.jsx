import React, { useMemo } from 'react';
import { Activity, AlertTriangle, Boxes, Cpu, Database, HardDrive, Lightbulb, Network, Server, ShieldCheck } from 'lucide-react';

const clampPercent = (value) => Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0));

function percent(value, total) {
  return total > 0 ? clampPercent((value / total) * 100) : 0;
}

function readyPair(value) {
  const match = String(value || '').match(/^(\d+)\/(\d+)$/);
  return match ? { ready: Number(match[1]), total: Number(match[2]) } : null;
}

function isPodReady(pod) {
  const pair = readyPair(pod.ready);
  return String(pod.phase || '').toLowerCase() === 'running' && (!pair || pair.ready >= pair.total);
}

function MetricBar({ label, value, total, valueLabel, tone = 'accent' }) {
  const ratio = percent(value, total);
  return (
    <div className="dashboard-metric-bar">
      <div className="dashboard-metric-head">
        <span>{label}</span>
        <strong>{valueLabel}</strong>
      </div>
      <div
        className="dashboard-progress-track"
        role="progressbar"
        aria-label={label}
        aria-valuemin="0"
        aria-valuemax={total || 100}
        aria-valuenow={value || 0}
      >
        <span className={`dashboard-progress-fill tone-${tone}`} style={{ width: `${ratio}%` }} />
      </div>
    </div>
  );
}

function SummaryCard({ icon: Icon, label, value, detail, tone = 'accent', loading = false }) {
  return (
    <div className={`dashboard-summary-card tone-${tone} ${loading ? 'is-loading' : ''}`} aria-busy={loading}>
      <div className="dashboard-summary-icon"><Icon size={16} strokeWidth={1.8} aria-hidden="true" /></div>
      <div className="dashboard-summary-copy">
        <span className="small-label">{label}</span>
        <strong>{loading ? '—' : value}</strong>
        <span>{loading ? 'Loading…' : detail}</span>
      </div>
    </div>
  );
}

function LoadingBlock({ label = 'Loading data…' }) {
  return (
    <div className="dashboard-loading-block" role="status">
      <span />
      <span />
      <small>{label}</small>
    </div>
  );
}

function Panel({ title, subtitle, icon: Icon, children, className = '' }) {
  return (
    <section className={`dashboard-panel ${className}`}>
      <div className="dashboard-panel-head">
        <div className="dashboard-panel-title">
          {Icon ? <Icon size={15} strokeWidth={1.8} aria-hidden="true" /> : null}
          <div>
            <h2>{title}</h2>
            {subtitle ? <p>{subtitle}</p> : null}
          </div>
        </div>
      </div>
      {children}
    </section>
  );
}

export default function DashboardPage({
  access,
  insightCategories,
  loading = {},
  selectedNamespaces,
  nodes,
  pods,
  workloads,
  services,
  ingresses,
  pvcs,
  pvs,
  storageClasses,
  events,
  insights,
  runningPodsHealth,
  formatMilliValue,
  formatByteValue
}) {
  const model = useMemo(() => {
    const readyNodes = nodes.filter((node) => String(node.status || '').trim().toLowerCase() === 'ready').length;
    const healthyPods = pods.filter(isPodReady).length;
    const podRestarts = pods.reduce((sum, pod) => sum + Number(pod.restarts || 0), 0);
    const podNamespaces = new Set(pods.map((pod) => pod.namespace).filter(Boolean)).size;
    const podStatuses = pods.reduce((counts, pod) => {
      const phase = String(pod.phase || 'Other').toLowerCase();
      const bucket = phase === 'running' && isPodReady(pod)
        ? 'healthy'
        : (phase === 'pending' ? 'pending' : (phase === 'succeeded' ? 'succeeded' : 'unhealthy'));
      counts[bucket] += 1;
      return counts;
    }, { healthy: 0, pending: 0, succeeded: 0, unhealthy: 0 });

    const workloadPairs = workloads.map((item) => readyPair(item.ready)).filter(Boolean);
    const workloadReady = workloadPairs.reduce((sum, item) => sum + item.ready, 0);
    const workloadDesired = workloadPairs.reduce((sum, item) => sum + item.total, 0);
    const readyWorkloads = workloads.filter((item) => {
      const pair = readyPair(item.ready);
      return !pair || pair.ready >= pair.total;
    }).length;

    const nodeMetrics = nodes.filter((node) => node.metrics_available && node.cpu_total_milli > 0 && node.memory_total_bytes > 0);
    const nodeCPUUsed = nodeMetrics.reduce((sum, node) => sum + Number(node.cpu_used_milli || 0), 0);
    const nodeCPUTotal = nodeMetrics.reduce((sum, node) => sum + Number(node.cpu_total_milli || 0), 0);
    const nodeMemoryUsed = nodeMetrics.reduce((sum, node) => sum + Number(node.memory_used_bytes || 0), 0);
    const nodeMemoryTotal = nodeMetrics.reduce((sum, node) => sum + Number(node.memory_total_bytes || 0), 0);

    const boundPVCs = pvcs.filter((pvc) => String(pvc.status || '').toLowerCase() === 'bound').length;
    const activeInsights = insights.filter((item) => item.status === 'alert' && !item.suppressed);
    const suppressedInsights = insights.filter((item) => item.status === 'alert' && item.suppressed);
    const passingInsights = insights.filter((item) => item.status !== 'alert');
    const criticalInsights = activeInsights.filter((item) => String(item.severity || '').toLowerCase() === 'critical').length;

    const categoryMap = new Map();
    activeInsights.forEach((item) => {
      const category = item.category || 'Other';
      categoryMap.set(category, (categoryMap.get(category) || 0) + 1);
    });
    const activeInsightCategories = [...categoryMap.entries()]
      .map(([label, count]) => ({ label, count }))
      .sort((left, right) => right.count - left.count || left.label.localeCompare(right.label));

    const typeMap = new Map();
    activeInsights.forEach((item) => {
      const key = item.check_type || item.check_label || item.category || 'Other';
      if (!typeMap.has(key)) typeMap.set(key, { label: item.check_label || item.category || key, count: 0, critical: 0 });
      const current = typeMap.get(key);
      current.count += 1;
      if (String(item.severity || '').toLowerCase() === 'critical') current.critical += 1;
    });
    const topInsightTypes = [...typeMap.values()]
      .sort((left, right) => right.critical - left.critical || right.count - left.count || left.label.localeCompare(right.label))
      .slice(0, 5);

    const scores = [];
    if (access.nodes && nodes.length > 0) scores.push(percent(readyNodes, nodes.length));
    if (access.pods && pods.length > 0) scores.push(percent(healthyPods, pods.length));
    if (access.workloads && workloads.length > 0) scores.push(percent(readyWorkloads, workloads.length));
    if (access.pvcs && pvcs.length > 0) scores.push(percent(boundPVCs, pvcs.length));
    if (access.insights && insightCategories.length > 0 && insights.length > 0) scores.push(percent(insights.length - activeInsights.length, insights.length));
    const healthScore = scores.length > 0 ? Math.round(scores.reduce((sum, score) => sum + score, 0) / scores.length) : null;

    const warningEvents = events
      .filter((event) => String(event.type || '').toLowerCase() === 'warning')
      .slice(0, 6);

    const topPods = runningPodsHealth
      .filter((pod) => pod.metrics_available)
      .slice()
      .sort((left, right) => (
        Number(right.cpu_used_milli || 0) + (Number(right.memory_used_bytes || 0) / (1024 * 1024))
        - Number(left.cpu_used_milli || 0) - (Number(left.memory_used_bytes || 0) / (1024 * 1024))
      ))
      .slice(0, 6);

    return {
      readyNodes,
      healthyPods,
      podRestarts,
      podNamespaces,
      podStatuses,
      workloadReady,
      workloadDesired,
      readyWorkloads,
      nodeCPUUsed,
      nodeCPUTotal,
      nodeMemoryUsed,
      nodeMemoryTotal,
      nodeMetricsCount: nodeMetrics.length,
      boundPVCs,
      activeInsights,
      suppressedInsights,
      passingInsights,
      criticalInsights,
      insightCategories: activeInsightCategories,
      topInsightTypes,
      healthScore,
      warningEvents,
      topPods
    };
  }, [access, events, insightCategories, insights, nodes, pods, pvcs, runningPodsHealth, workloads]);

  const canShowInsights = access.insights && insightCategories.length > 0;
  const visibleSignals = ['nodes', 'pods', 'workloads', 'events', 'services', 'ingresses', 'pvcs', 'pvs', 'storageclasses']
    .filter((resource) => access[resource]).length + (canShowInsights ? 1 : 0);
  const maxInsightCategory = Math.max(1, ...model.insightCategories.map((item) => item.count));

  return (
    <div className="dashboard-page">
      <div className="dashboard-intro">
        <div>
          <span className="small-label">Cluster overview</span>
          <h1>Operational dashboard</h1>
          <p>Live status for {selectedNamespaces.length || 0} selected namespace{selectedNamespaces.length === 1 ? '' : 's'}, scoped to resources your role can view.</p>
        </div>
        <div className="dashboard-scope-badge">
          <ShieldCheck size={15} strokeWidth={1.8} aria-hidden="true" />
          RBAC scoped · {visibleSignals} data source{visibleSignals === 1 ? '' : 's'}
        </div>
      </div>

      {selectedNamespaces.length === 0 ? (
        <div className="dashboard-notice">
          Select at least one namespace to include namespaced resources. Cluster-scoped signals remain visible when permitted.
        </div>
      ) : null}

      <div className="dashboard-hero-grid">
        <section className="dashboard-score-card">
          <div
            className={`dashboard-score-ring ${model.healthScore == null ? 'is-empty' : ''}`}
            role="img"
            aria-label={model.healthScore == null ? 'Health score unavailable' : `Visible health score ${model.healthScore} percent`}
          >
            <svg viewBox="0 0 120 120" aria-hidden="true">
              <circle className="dashboard-score-track" cx="60" cy="60" r="49" pathLength="100" />
              <circle className="dashboard-score-value" cx="60" cy="60" r="49" pathLength="100" style={{ strokeDashoffset: 100 - (model.healthScore || 0) }} />
            </svg>
            <div>
              <strong>{model.healthScore == null ? '—' : model.healthScore}</strong>
              <span>{model.healthScore == null ? 'No signals' : '/ 100'}</span>
            </div>
          </div>
          <div className="dashboard-score-copy">
            <span className="small-label">Visible health score</span>
            <h2>{model.healthScore == null ? 'Waiting for accessible data' : (model.healthScore >= 90 ? 'Cluster looks healthy' : (model.healthScore >= 70 ? 'Attention recommended' : 'Action required'))}</h2>
            <p>Calculated only from ready nodes, healthy pods, workload readiness, bound PVCs, and Insights visible to this role.</p>
          </div>
        </section>

        <div className="dashboard-summary-grid">
          {access.nodes ? <SummaryCard icon={Server} label="Nodes" value={nodes.length} detail={`${model.readyNodes} ready · ${nodes.length - model.readyNodes} not ready`} tone={nodes.length > model.readyNodes ? 'danger' : 'ok'} loading={loading.nodes} /> : null}
          {access.pods ? <SummaryCard icon={Boxes} label="Pods" value={pods.length} detail={`${model.healthyPods} healthy · ${pods.length - model.healthyPods} need attention`} tone={pods.length > model.healthyPods ? 'warn' : 'ok'} loading={loading.pods} /> : null}
          {access.workloads ? <SummaryCard icon={Activity} label="Workloads" value={workloads.length} detail={`${model.readyWorkloads} ready · ${workloads.length - model.readyWorkloads} degraded`} tone={workloads.length > model.readyWorkloads ? 'warn' : 'ok'} loading={loading.workloads} /> : null}
          {canShowInsights ? <SummaryCard icon={Lightbulb} label="Active insights" value={model.activeInsights.length} detail={`${model.criticalInsights} critical · ${model.suppressedInsights.length} suppressed`} tone={model.criticalInsights > 0 ? 'danger' : (model.activeInsights.length > 0 ? 'warn' : 'ok')} loading={loading.insights} /> : null}
        </div>
      </div>

      <div className="dashboard-main-grid">
        {(access.nodes || access.pods || access.workloads || access.pvcs) ? (
          <Panel title="Capacity & readiness" subtitle="Current utilization and object readiness" icon={Cpu}>
            <div className="dashboard-bars">
              {access.nodes && model.nodeCPUTotal > 0 ? (
                <MetricBar label="Node CPU" value={model.nodeCPUUsed} total={model.nodeCPUTotal} valueLabel={`${formatMilliValue(model.nodeCPUUsed)} / ${formatMilliValue(model.nodeCPUTotal)}`} tone={percent(model.nodeCPUUsed, model.nodeCPUTotal) > 85 ? 'danger' : 'accent'} />
              ) : null}
              {access.nodes && model.nodeMemoryTotal > 0 ? (
                <MetricBar label="Node memory" value={model.nodeMemoryUsed} total={model.nodeMemoryTotal} valueLabel={`${formatByteValue(model.nodeMemoryUsed)} / ${formatByteValue(model.nodeMemoryTotal)}`} tone={percent(model.nodeMemoryUsed, model.nodeMemoryTotal) > 85 ? 'danger' : 'accent'} />
              ) : null}
              {access.workloads && model.workloadDesired > 0 ? (
                <MetricBar label="Workload replicas" value={model.workloadReady} total={model.workloadDesired} valueLabel={`${model.workloadReady} / ${model.workloadDesired} ready`} tone={model.workloadReady < model.workloadDesired ? 'warn' : 'ok'} />
              ) : null}
              {access.pvcs && pvcs.length > 0 ? (
                <MetricBar label="Bound PVCs" value={model.boundPVCs} total={pvcs.length} valueLabel={`${model.boundPVCs} / ${pvcs.length} bound`} tone={model.boundPVCs < pvcs.length ? 'warn' : 'ok'} />
              ) : null}
              {(loading.nodes || loading.workloads || loading.pvcs) ? <LoadingBlock label="Loading remaining capacity signals…" /> : null}
              {access.nodes && nodes.length > 0 && model.nodeMetricsCount === 0 ? <div className="dashboard-inline-empty">Node metrics are unavailable.</div> : null}
            </div>
          </Panel>
        ) : null}

        {access.pods ? (
          <Panel title="Pod health" subtitle="Status distribution in selected namespaces" icon={Activity}>
            {loading.pods && pods.length === 0 ? <LoadingBlock label="Loading pod health…" /> : (pods.length > 0 ? (
              <>
                <div className="dashboard-stacked-bar" role="img" aria-label={`${model.podStatuses.healthy} healthy, ${model.podStatuses.pending} pending, ${model.podStatuses.succeeded} succeeded, ${model.podStatuses.unhealthy} unhealthy pods`}>
                  {Object.entries(model.podStatuses).map(([status, count]) => count > 0 ? (
                    <span key={status} className={`status-${status}`} style={{ width: `${percent(count, pods.length)}%` }} title={`${status}: ${count}`} />
                  ) : null)}
                </div>
                <div className="dashboard-legend">
                  <span className="status-healthy">Healthy <strong>{model.podStatuses.healthy}</strong></span>
                  <span className="status-pending">Pending <strong>{model.podStatuses.pending}</strong></span>
                  <span className="status-succeeded">Succeeded <strong>{model.podStatuses.succeeded}</strong></span>
                  <span className="status-unhealthy">Unhealthy <strong>{model.podStatuses.unhealthy}</strong></span>
                </div>
                <div className="dashboard-pod-kpis">
                  <div><span>Ready</span><strong>{model.healthyPods} / {pods.length}</strong></div>
                  <div><span>Need attention</span><strong>{pods.length - model.healthyPods}</strong></div>
                  <div><span>Restarts</span><strong>{model.podRestarts}</strong></div>
                  <div><span>Namespaces</span><strong>{model.podNamespaces}</strong></div>
                </div>
              </>
            ) : <div className="dashboard-inline-empty">No pods found in the selected namespaces.</div>)}
          </Panel>
        ) : null}

        {canShowInsights ? (
          <Panel title="Insights summary" subtitle={`Active findings across ${insightCategories.length} accessible categor${insightCategories.length === 1 ? 'y' : 'ies'}`} icon={Lightbulb} className="dashboard-panel-wide">
            {loading.insights && insights.length === 0 ? <LoadingBlock label="Scanning accessible Insight categories…" /> : <div className="dashboard-insights-layout">
              <div className="dashboard-insight-totals">
                <div><strong>{model.activeInsights.length}</strong><span>Active</span></div>
                <div><strong>{model.criticalInsights}</strong><span>Critical</span></div>
                <div><strong>{model.passingInsights.length}</strong><span>Passing</span></div>
                <div><strong>{model.suppressedInsights.length}</strong><span>Suppressed</span></div>
              </div>
              <div className="dashboard-category-chart">
                {model.insightCategories.length > 0 ? model.insightCategories.map((category) => (
                  <div key={category.label} className="dashboard-category-row">
                    <span title={category.label}>{category.label}</span>
                    <div className="dashboard-category-track"><i style={{ width: `${percent(category.count, maxInsightCategory)}%` }} /></div>
                    <strong>{category.count}</strong>
                  </div>
                )) : <div className="dashboard-inline-empty">No active Insights in the accessible categories.</div>}
              </div>
              {model.topInsightTypes.length > 0 ? (
                <div className="dashboard-top-findings">
                  <span className="small-label">Top findings</span>
                  {model.topInsightTypes.map((item) => (
                    <div key={item.label}><span>{item.label}</span><strong className={item.critical > 0 ? 'text-danger' : ''}>{item.count}</strong></div>
                  ))}
                </div>
              ) : null}
            </div>}
          </Panel>
        ) : null}

        {access.pods ? (
          <Panel title="Top pod consumers" subtitle="Running pods with available metrics" icon={Cpu}>
            {loading.pods && model.topPods.length === 0 ? <LoadingBlock label="Loading pod metrics…" /> : (model.topPods.length > 0 ? (
              <div className="dashboard-consumer-list">
                {model.topPods.map((pod) => {
                  const cpuTarget = Number(pod.cpu_limit_milli || pod.cpu_request_milli || pod.cpu_total_milli || 0);
                  const memoryTarget = Number(pod.memory_limit_bytes || pod.memory_request_bytes || pod.memory_total_bytes || 0);
                  return (
                    <div key={`${pod.namespace}/${pod.name}`} className="dashboard-consumer-row">
                      <div className="dashboard-consumer-name"><strong title={pod.name}>{pod.name}</strong><span>{pod.namespace}</span></div>
                      <div className="dashboard-consumer-metrics">
                        <MetricBar label="CPU" value={Number(pod.cpu_used_milli || 0)} total={cpuTarget} valueLabel={formatMilliValue(pod.cpu_used_milli)} tone={percent(pod.cpu_used_milli, cpuTarget) > 85 ? 'danger' : 'accent'} />
                        <MetricBar label="Memory" value={Number(pod.memory_used_bytes || 0)} total={memoryTarget} valueLabel={formatByteValue(pod.memory_used_bytes)} tone={percent(pod.memory_used_bytes, memoryTarget) > 85 ? 'danger' : 'accent'} />
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : <div className="dashboard-inline-empty">Pod metrics are unavailable or no running pods were found.</div>)}
          </Panel>
        ) : null}

        {access.events ? (
          <Panel title="Recent warning events" subtitle="Latest Kubernetes warnings in scope" icon={AlertTriangle}>
            {loading.events && model.warningEvents.length === 0 ? <LoadingBlock label="Loading warning events…" /> : (model.warningEvents.length > 0 ? (
              <div className="dashboard-event-list">
                {model.warningEvents.map((event, index) => (
                  <div key={`${event.namespace}/${event.object}/${event.reason}/${index}`} className="dashboard-event-row">
                    <AlertTriangle size={14} strokeWidth={1.8} aria-hidden="true" />
                    <div><strong>{event.reason || 'Warning'}</strong><span>{event.object || 'Kubernetes object'} · {event.namespace || 'cluster'}</span></div>
                    <time>{event.last_seen || 'recently'}</time>
                  </div>
                ))}
              </div>
            ) : <div className="dashboard-inline-empty">No warning events are currently loaded.</div>)}
          </Panel>
        ) : null}

        {(access.services || access.ingresses || access.pvcs || access.pvs || access.storageclasses) ? (
          <Panel title="Resource inventory" subtitle="Objects visible in the current scope" icon={Database} className="dashboard-panel-wide">
            <div className="dashboard-inventory-grid">
              {access.services ? <div className={loading.services ? 'is-loading' : ''}><Network size={14} aria-hidden="true" /><span>Services</span><strong>{loading.services ? '—' : services.length}</strong></div> : null}
              {access.ingresses ? <div className={loading.ingresses ? 'is-loading' : ''}><Network size={14} aria-hidden="true" /><span>Ingresses</span><strong>{loading.ingresses ? '—' : ingresses.length}</strong></div> : null}
              {access.pvcs ? <div className={loading.pvcs ? 'is-loading' : ''}><HardDrive size={14} aria-hidden="true" /><span>PVCs</span><strong>{loading.pvcs ? '—' : pvcs.length}</strong></div> : null}
              {access.pvs ? <div className={loading.pvs ? 'is-loading' : ''}><HardDrive size={14} aria-hidden="true" /><span>PVs</span><strong>{loading.pvs ? '—' : pvs.length}</strong></div> : null}
              {access.storageclasses ? <div className={loading.storageclasses ? 'is-loading' : ''}><Database size={14} aria-hidden="true" /><span>StorageClasses</span><strong>{loading.storageclasses ? '—' : storageClasses.length}</strong></div> : null}
            </div>
          </Panel>
        ) : null}
      </div>

      {visibleSignals === 0 ? (
        <div className="dashboard-empty-access">
          <ShieldCheck size={24} strokeWidth={1.6} aria-hidden="true" />
          <div><strong>Dashboard is available</strong><span>No dashboard data sources are visible to this role yet. Panels will appear as View access is granted.</span></div>
        </div>
      ) : null}
    </div>
  );
}
