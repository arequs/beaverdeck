import { test } from 'node:test';
import assert from 'node:assert/strict';
import { activeNavigation, eventNamespacesFromSearch, isRoutedNavigation, namespaceSelectionMatches, navigationPath } from './navigation.js';

test('maps routed and transitional workspace paths to navigation', () => {
  for (const path of ['/', '/dashboard', '/login']) assert.equal(activeNavigation(path, 'pods'), 'dashboard');
  assert.equal(activeNavigation('/events', 'pods'), 'events');
  assert.equal(activeNavigation('/_workspace', 'pods'), 'pods');
  assert.equal(activeNavigation('/unknown', 'pods'), 'not-found');
});

test('selects dedicated routes for migrated pages and the shared workspace for other pages', () => {
  for (const [navId, path] of [['dashboard', '/dashboard'], ['events', '/events']]) {
    assert.equal(isRoutedNavigation(navId), true);
    assert.equal(navigationPath(navId), path);
    assert.equal(activeNavigation(path, ''), navId);
  }
  assert.equal(isRoutedNavigation('pods'), false);
  assert.equal(navigationPath('pods'), '/_workspace');
});

test('distinguishes absent and empty namespace filters and preserves selection order', () => {
  assert.equal(eventNamespacesFromSearch(new URLSearchParams()), null);
  assert.deepEqual(eventNamespacesFromSearch(new URLSearchParams('namespace=')), []);
  const requested = eventNamespacesFromSearch(new URLSearchParams('namespace=prod%2C+dev%2Cprod'));
  assert.deepEqual(requested, ['prod', 'dev']);
  assert.equal(namespaceSelectionMatches(requested, ['prod', 'dev']), true);
  assert.equal(namespaceSelectionMatches(requested, ['dev', 'prod']), false);
});
