import React from 'react';
import { Link } from 'react-router';

function RouteMessage({ title, message }) {
  return (
    <div className="nav-loading-state">
      <strong>{title}</strong>
      <span>{message}</span>
      <Link to="/">Go to Dashboard</Link>
    </div>
  );
}

export function ForbiddenPage() {
  return <RouteMessage title="Access denied" message="You do not have access to this page or namespace." />;
}

export function NotFoundPage() {
  return <RouteMessage title="Page not found" message="The requested page does not exist." />;
}
