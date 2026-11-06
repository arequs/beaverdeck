import { test } from 'node:test';
import assert from 'node:assert/strict';
import { activeNavigation, eventNamespacesFromSearch, namespaceSelectionMatches } from './navigation.js';

test('maps routed and transitional workspace paths to navigation', () => {
  for (const path of ['/', '/dashboard', '/login']) assert.equal(activeNavigation(path, 'pods'), 'dashboard');
  assert.equal(activeNavigation('/events', 'pods'), 'events');
  assert.equal(activeNavigation('/_workspace', 'pods'), 'pods');
  assert.equal(activeNavigation('/unknown', 'pods'), 'not-found');
});

test('distinguishes absent and empty namespace filters and preserves selection order', () => {
  assert.equal(eventNamespacesFromSearch(new URLSearchParams()), null);
  assert.deepEqual(eventNamespacesFromSearch(new URLSearchParams('namespace=')), []);
  const requested = eventNamespacesFromSearch(new URLSearchParams('namespace=prod%2C+dev%2Cprod'));
  assert.deepEqual(requested, ['prod', 'dev']);
  assert.equal(namespaceSelectionMatches(requested, ['prod', 'dev']), true);
  assert.equal(namespaceSelectionMatches(requested, ['dev', 'prod']), false);
});
