function normalizeBasePath(path) {
  if (!path || path === '/') return '';
  return `/${String(path).replace(/^\/+|\/+$/g, '')}`;
}

export function appBasePath() {
  if (typeof window === 'undefined') return '';

  const viteBase = import.meta.env.BASE_URL;
  if (viteBase && viteBase !== '/' && viteBase !== './') {
    return normalizeBasePath(viteBase);
  }

  const pathname = window.location.pathname || '/';
  if (pathname === '/') return '';
  if (pathname.endsWith('/')) return normalizeBasePath(pathname);

  const segments = pathname.split('/');
  const last = segments[segments.length - 1] || '';
  if (last.includes('.')) {
    return normalizeBasePath(segments.slice(0, -1).join('/'));
  }
  return normalizeBasePath(pathname);
}

export function withBasePath(path) {
  const basePath = appBasePath();
  const normalizedPath = String(path || '');
  if (!normalizedPath.startsWith('/')) {
    return normalizedPath;
  }
  return `${basePath}${normalizedPath}`;
}

