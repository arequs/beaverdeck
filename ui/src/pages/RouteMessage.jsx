import React from 'react';

export function ForbiddenPage() {
  return <div className="nav-loading-state"><strong>Access denied</strong><span>You do not have access to this page or namespace.</span></div>;
}

export function NotFoundPage() {
  return <div className="nav-loading-state"><strong>Page not found</strong><span>The requested page does not exist.</span></div>;
}
