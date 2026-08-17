import React from 'react';
import ActionMenu from './ActionMenu.jsx';

function displayTimestamp(value) {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function ApplicationStatus({ status }) {
  const normalized = String(status || 'unknown').toLowerCase();
  return <span className="application-status" data-status={normalized}>{status || 'unknown'}</span>;
}

export default function ArgoCDApplicationsPage({
  applicationSearch,
  setApplicationSearch,
  sortedApplications,
  selectedApplication,
  applicationHistory,
  toggleSort,
  sortMark,
  makeAction,
  permissionInfo,
  safe,
  openHistory,
  closeHistory,
  openRevisionDetail
}) {
  if (selectedApplication) {
    return (
      <>
        <div className="toolbar fixed-toolbar">
          <button className="secondary" onClick={closeHistory}>All applications</button>
          <strong>{selectedApplication.name}</strong>
          <span className="small-hint">{selectedApplication.namespace}</span>
          <span className="small-hint">Sync</span>
          <ApplicationStatus status={selectedApplication.sync_status} />
          <span className="small-hint">Health</span>
          <ApplicationStatus status={selectedApplication.health_status} />
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>History ID</th>
                <th>Revision</th>
                <th>Status</th>
                <th>Source</th>
                <th>Deploy Started</th>
                <th>Deployed</th>
                <th>Initiated By</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {applicationHistory.map((revision) => {
                const manage = permissionInfo('applications', 'edit', revision.namespace);
                const resources = revision.current
                  ? manage
                  : { allowed: false, reason: 'Created resources are available only for the current revision' };
                return (
                  <tr key={`${revision.namespace}/${revision.name}/${revision.id}`}>
                    <td>{revision.id}</td>
                    <td className="mono-cell">{revision.revision || '-'}</td>
                    <td><ApplicationStatus status={revision.current ? 'Current' : 'Historical'} /></td>
                    <td>{revision.source || '-'}</td>
                    <td>{displayTimestamp(revision.deploy_started)}</td>
                    <td>{displayTimestamp(revision.deployed)}</td>
                    <td>{revision.initiated_by || '-'}</td>
                    <td className="actions-cell">
                      <ActionMenu actions={[
                        makeAction('Source configuration', manage, () => safe(() => openRevisionDetail(revision, 'source'))),
                        makeAction('Created resources', resources, () => safe(() => openRevisionDetail(revision, 'resources')))
                      ]} />
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          {applicationHistory.length === 0 ? <div className="empty-state">No deployment history found.</div> : null}
        </div>
      </>
    );
  }

  return (
    <>
      <div className="toolbar fixed-toolbar">
        <input value={applicationSearch} onChange={(event) => setApplicationSearch(event.target.value)} placeholder="Search Argo CD applications..." />
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th><button className="sort-btn" onClick={() => toggleSort('argocdapplications', 'name')}>Name {sortMark('argocdapplications', 'name')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('argocdapplications', 'namespace')}>Namespace {sortMark('argocdapplications', 'namespace')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('argocdapplications', 'project')}>Project {sortMark('argocdapplications', 'project')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('argocdapplications', 'sync_status')}>Sync {sortMark('argocdapplications', 'sync_status')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('argocdapplications', 'health_status')}>Health {sortMark('argocdapplications', 'health_status')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('argocdapplications', 'revision')}>Revision {sortMark('argocdapplications', 'revision')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('argocdapplications', 'source')}>Source {sortMark('argocdapplications', 'source')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('argocdapplications', 'destination')}>Destination {sortMark('argocdapplications', 'destination')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('argocdapplications', 'updated')}>Updated {sortMark('argocdapplications', 'updated')}</button></th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {sortedApplications.map((application) => (
              <tr key={`${application.namespace}/${application.name}`}>
                <td>{application.name}</td>
                <td>{application.namespace}</td>
                <td>{application.project || '-'}</td>
                <td><ApplicationStatus status={application.sync_status} /></td>
                <td><ApplicationStatus status={application.health_status} /></td>
                <td className="mono-cell">{application.revision || '-'}</td>
                <td>{application.source || '-'}</td>
                <td>{application.destination || '-'}</td>
                <td>{displayTimestamp(application.updated)}</td>
                <td className="actions-cell">
                  <ActionMenu actions={[
                    makeAction('History', permissionInfo('applications', 'view', application.namespace), () => safe(() => openHistory(application)))
                  ]} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {sortedApplications.length === 0 ? <div className="empty-state">No Argo CD applications found.</div> : null}
      </div>
    </>
  );
}
