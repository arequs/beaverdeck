import { appBasePath } from './paths.js';
import { AUTH_API_ROOT, LOGIN_ROUTE } from '../shared/routes/index.js';

const FORBIDDEN_DESTINATION_CHARACTERS = /[\\\u0000-\u001f\u007f#]/;
const FORBIDDEN_PATH_CHARACTERS = /[\\\u0000-\u001f\u007f?#]/;

// Accept only internal browser paths, including the deployment prefix.
// Return the original destination so ordinary URL-encoded query values are preserved.
export function validateReturnTo(destination, basePath = appBasePath()) {
  if (
    typeof destination !== 'string' ||
    !destination.startsWith('/') ||
    destination.startsWith('//') ||
    FORBIDDEN_DESTINATION_CHARACTERS.test(destination)
  ) {
    return null;
  }

  const [rawPath] = destination.split('?');
  let decodedPath;

  // Decode the path before checking its characters, segments, and route.
  try {
    decodedPath = decodeURIComponent(rawPath);
  } catch {
    // Malformed percent encoding cannot be safely interpreted as a path.
    return null;
  }

  // Reject unsafe characters and ambiguous separators in the decoded path.
  if (FORBIDDEN_PATH_CHARACTERS.test(decodedPath) || decodedPath.includes('//')) {
    return null;
  }

  const hasTraversalSegment = decodedPath.split('/').some((segment) => segment === '.' || segment === '..');
  if (hasTraversalSegment) {
    return null;
  }

  // Match the whole prefix or a following slash, not a similarly named prefix.
  const isWithinBasePath = !basePath || decodedPath === basePath || decodedPath.startsWith(`${basePath}/`);
  if (!isWithinBasePath) {
    return null;
  }

  const appRelativePath = decodedPath.slice(basePath.length) || '/';
  // Returning to login or an auth endpoint could create an authentication loop.
  const normalizedPath = appRelativePath.toLowerCase();
  const isAuthPath = [LOGIN_ROUTE, AUTH_API_ROOT].some((route) => {
    const normalizedRoute = route.toLowerCase();
    return normalizedPath === normalizedRoute || normalizedPath.startsWith(`${normalizedRoute}/`);
  });
  if (isAuthPath) {
    return null;
  }

  return destination;
}

export function loginDestination() {
  const params = new URLSearchParams(window.location.search);
  return validateReturnTo(params.get('returnTo'));
}

export function currentDestination() {
  const base = appBasePath();
  if (window.location.pathname === `${base}${LOGIN_ROUTE}`) return loginDestination();
  return validateReturnTo(`${window.location.pathname}${window.location.search}`);
}

export function oauthDestinationKey() {
  return `beaverdeck-return-to:${appBasePath() || '/'}`;
}

export function saveOAuthDestination(destination) {
  try {
    const value = validateReturnTo(destination);
    if (value) sessionStorage.setItem(oauthDestinationKey(), value);
    else sessionStorage.removeItem(oauthDestinationKey());
  } catch { /* Navigation metadata is optional. */ }
}

export function readOAuthDestination(remove = false) {
  try {
    const value = validateReturnTo(sessionStorage.getItem(oauthDestinationKey()));
    if (remove) sessionStorage.removeItem(oauthDestinationKey());
    return value;
  } catch { return null; }
}
