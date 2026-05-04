import { withBasePath } from './paths.js';

export const AUTH_EXPIRED_EVENT = 'beaverdeck-auth-expired';

async function readApiResponse(response) {
  if (!response.ok) {
    let errorText = await response.text();
    try {
      const parsed = JSON.parse(errorText);
      errorText = parsed.error || errorText;
    } catch {
      // Keep raw text when the backend returns non-JSON errors.
    }
    throw new Error(errorText || `HTTP ${response.status}`);
  }

  const contentType = response.headers.get('content-type') || '';
  if (contentType.includes('application/json')) {
    return response.json();
  }
  return response.text();
}

export async function publicApi(path, options = {}) {
  const response = await fetch(withBasePath(path), options);
  return readApiResponse(response);
}

export function createApi(token, username) {
  return async function api(path, options = {}) {
    const headers = {
      ...(options.headers || {}),
      Authorization: `Bearer ${token}`,
      'X-BeaverDeck-Username': username
    };
    const response = await fetch(withBasePath(path), { ...options, headers });
    if (response.status === 401 && typeof window !== 'undefined') {
      let errorText = 'Session expired. Please sign in again.';
      try {
        const parsed = await response.clone().json();
        errorText = parsed.error || errorText;
      } catch {
        const raw = await response.clone().text();
        if (raw) {
          errorText = raw;
        }
      }
      window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT, { detail: { message: errorText } }));
    }
    return readApiResponse(response);
  };
}
