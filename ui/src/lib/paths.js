function normalizeBasePath(path) {
  if (!path || path === '/') return '';
  return `/${String(path).replace(/^\/+|\/+$/g, '')}`;
}

export function appBasePath() {
  if (typeof window === 'undefined') return '';

  const runtimeBase = document.querySelector('meta[name="beaverdeck-base-path"]');
  if (runtimeBase) return normalizeBasePath(runtimeBase.content);

  const viteBase = import.meta.env.BASE_URL;
  if (viteBase && viteBase !== '/' && viteBase !== './') {
    return normalizeBasePath(viteBase);
  }

  return '';
}

export function withBasePath(path) {
  const basePath = appBasePath();
  const normalizedPath = String(path || '');
  if (!normalizedPath.startsWith('/')) {
    return normalizedPath;
  }
  return `${basePath}${normalizedPath}`;
}

