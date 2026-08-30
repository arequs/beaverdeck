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
  async function authenticatedFetch(path, options = {}) {
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
    return response;
  }

  const api = async function api(path, options = {}) {
    const response = await authenticatedFetch(path, options);
    return readApiResponse(response);
  };

  api.stream = async function stream(path, options = {}) {
    const response = await authenticatedFetch(path, options);
    if (!response.ok) {
      await readApiResponse(response);
    }
    if (!response.body) {
      throw new Error('Streaming response body is unavailable');
    }
    return response.body;
  };

  return api;
}
