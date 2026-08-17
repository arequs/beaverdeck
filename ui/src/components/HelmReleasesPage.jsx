import React from 'react';
import ActionMenu from './ActionMenu.jsx';

function displayTimestamp(value) {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function ReleaseStatus({ status }) {
  const normalized = String(status || 'unknown').toLowerCase();
  return <span className="helm-status" data-status={normalized}>{status || 'unknown'}</span>;
}

export default function HelmReleasesPage({
  releaseSearch,
  setReleaseSearch,
  sortedReleases,
  selectedRelease,
  releaseHistory,
  toggleSort,
  sortMark,
  makeAction,
  permissionInfo,
  safe,
  openHistory,
  closeHistory,
  openRevisionDetail
}) {
  if (selectedRelease) {
    return (
      <>
        <div className="toolbar fixed-toolbar">
          <button className="secondary" onClick={closeHistory}>All releases</button>
          <strong>{selectedRelease.name}</strong>
          <span className="small-hint">{selectedRelease.namespace}</span>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Revision</th>
                <th>Status</th>
                <th>Chart</th>
                <th>Chart Version</th>
                <th>App Version</th>
                <th>Updated</th>
                <th>Description</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {releaseHistory.map((revision) => (
                <tr key={`${revision.namespace}/${revision.name}/${revision.revision}`}>
                  <td>{revision.revision}</td>
                  <td><ReleaseStatus status={revision.status} /></td>
                  <td>{revision.chart || '-'}</td>
                  <td>{revision.chart_version || '-'}</td>
                  <td>{revision.app_version || '-'}</td>
                  <td>{displayTimestamp(revision.updated)}</td>
                  <td>{revision.description || '-'}</td>
                  <td className="actions-cell">
                    <ActionMenu actions={[
                      makeAction('Values', permissionInfo('applications', 'edit', revision.namespace), () => safe(() => openRevisionDetail(revision, 'values'))),
                      makeAction('User-applied values', permissionInfo('applications', 'edit', revision.namespace), () => safe(() => openRevisionDetail(revision, 'user-values'))),
                      makeAction('Created resources', permissionInfo('applications', 'edit', revision.namespace), () => safe(() => openRevisionDetail(revision, 'resources')))
                    ]} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {releaseHistory.length === 0 ? <div className="empty-state">No release history found.</div> : null}
        </div>
      </>
    );
  }

  return (
    <>
      <div className="toolbar fixed-toolbar">
        <input value={releaseSearch} onChange={(event) => setReleaseSearch(event.target.value)} placeholder="Search Helm releases..." />
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th><button className="sort-btn" onClick={() => toggleSort('helmreleases', 'name')}>Name {sortMark('helmreleases', 'name')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('helmreleases', 'namespace')}>Namespace {sortMark('helmreleases', 'namespace')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('helmreleases', 'status')}>Status {sortMark('helmreleases', 'status')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('helmreleases', 'revision')}>Revision {sortMark('helmreleases', 'revision')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('helmreleases', 'chart')}>Chart {sortMark('helmreleases', 'chart')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('helmreleases', 'chart_version')}>Chart Version {sortMark('helmreleases', 'chart_version')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('helmreleases', 'app_version')}>App Version {sortMark('helmreleases', 'app_version')}</button></th>
              <th><button className="sort-btn" onClick={() => toggleSort('helmreleases', 'updated')}>Updated {sortMark('helmreleases', 'updated')}</button></th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {sortedReleases.map((release) => (
              <tr key={`${release.namespace}/${release.name}`}>
                <td>{release.name}</td>
                <td>{release.namespace}</td>
                <td><ReleaseStatus status={release.status} /></td>
                <td>{release.revision}</td>
                <td>{release.chart || '-'}</td>
                <td>{release.chart_version || '-'}</td>
                <td>{release.app_version || '-'}</td>
                <td>{displayTimestamp(release.updated)}</td>
                <td className="actions-cell">
                  <ActionMenu actions={[
                    makeAction('History', permissionInfo('applications', 'view', release.namespace), () => safe(() => openHistory(release)))
                  ]} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {sortedReleases.length === 0 ? <div className="empty-state">No Helm releases found.</div> : null}
      </div>
    </>
  );
}
