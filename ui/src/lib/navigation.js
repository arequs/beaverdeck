import { matchPath } from 'react-router';
import { DASHBOARD_ROUTE, EVENTS_ROUTE, LOGIN_ROUTE, WORKSPACE_ROUTE } from '../shared/routes/index.js';

// Workspace pages share a URL until each page has its own route.
export function activeNavigation(pathname, workspaceNav) {
  if (matchPath(EVENTS_ROUTE, pathname)) return 'events';
  if ([DASHBOARD_ROUTE, '/', LOGIN_ROUTE].some((route) => matchPath(route, pathname))) return 'dashboard';
  if (matchPath(WORKSPACE_ROUTE, pathname)) return workspaceNav;
  return 'not-found';
}

// null means the URL has no explicit namespace selection; [] is an explicit empty selection.
export function eventNamespacesFromSearch(params) {
  if (!params.has('namespace')) return null;
  return [...new Set(params.get('namespace').split(',').map((namespace) => namespace.trim()).filter(Boolean))];
}

export function namespaceSelectionMatches(requested, selected) {
  return requested.length === selected.length && requested.every((namespace, index) => namespace === selected[index]);
}
