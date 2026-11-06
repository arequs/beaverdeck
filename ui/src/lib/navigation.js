import { matchPath } from 'react-router';
import { DASHBOARD_ROUTE, EVENTS_ROUTE, LOGIN_ROUTE, WORKSPACE_ROUTE } from '../shared/routes/index.js';

// Add migrated pages here so navigation and URL matching use the same routes.
const ROUTED_NAVIGATION = new Map([
  ['dashboard', DASHBOARD_ROUTE],
  ['events', EVENTS_ROUTE]
]);

export function isRoutedNavigation(navId) {
  return ROUTED_NAVIGATION.has(navId);
}

export function navigationPath(navId) {
  return ROUTED_NAVIGATION.get(navId) ?? WORKSPACE_ROUTE;
}

// Workspace pages share a URL until each page has its own route.
export function activeNavigation(pathname, workspaceNav) {
  for (const [navId, route] of ROUTED_NAVIGATION) {
    if (matchPath(route, pathname)) return navId;
  }
  if (matchPath('/', pathname) || matchPath(LOGIN_ROUTE, pathname)) return 'dashboard';
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
