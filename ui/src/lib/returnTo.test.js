import { test } from 'node:test';
import assert from 'node:assert/strict';
import { validateReturnTo, saveOAuthDestination, readOAuthDestination } from './returnTo.js';
import { AUTH_API_ROOT, AUTH_API_ROUTES, LOGIN_ROUTE } from '../shared/routes/index.js';

test('accepts internal destinations and keeps encoded search under a prefix', () => {
  assert.equal(validateReturnTo('/beaverdeck/events?namespace=production&q=a%2Fb%20c', '/beaverdeck'), '/beaverdeck/events?namespace=production&q=a%2Fb%20c');
  assert.equal(validateReturnTo('/beaverdeck/unknown', '/beaverdeck'), '/beaverdeck/unknown');
});

test('rejects external URLs, path escapes, unsafe characters and login loops', () => {
  for (const value of ['https://evil.test/', '//evil.test', '/beaverdeck-other/events', '/beaverdeck/%2e%2e/evil', '/beaverdeck/%2f%2fevil', '/beaverdeck/%5cevil', '/beaverdeck/\nfoo', '/beaverdeck/%00foo', '/beaverdeck/login', '/beaverdeck/%6cogin', '/beaverdeck/api/auth/login', '/beaverdeck/events#auth_session=secret']) {
    assert.equal(validateReturnTo(value, '/beaverdeck'), null, value);
  }
});

test('allows encoded path separators and preserves query parameters', () => {
  for (const value of ['/beaverdeck/events%2Fdetails', '/beaverdeck/events?token=example']) {
    assert.equal(validateReturnTo(value, '/beaverdeck'), value);
  }
});

test('shared auth routes and their descendants are not return destinations', () => {
  for (const route of [LOGIN_ROUTE, AUTH_API_ROOT, ...Object.values(AUTH_API_ROUTES)]) {
    assert.equal(validateReturnTo(route, ''), null, route);
    assert.equal(validateReturnTo(`${route.toUpperCase()}/child`, ''), null, route);
  }
  assert.equal(validateReturnTo(`${LOGIN_ROUTE}-help`, ''), `${LOGIN_ROUTE}-help`);
  assert.equal(validateReturnTo(`${AUTH_API_ROOT}ors`, ''), `${AUTH_API_ROOT}ors`);
});

test('storage contains only validated metadata and blocked storage is harmless', () => {
  global.window = {};
  global.document = { querySelector: () => ({ content: '/beaverdeck' }) };
  const data = new Map();
  global.sessionStorage = { setItem: (key, value) => data.set(key, value), getItem: (key) => data.get(key), removeItem: (key) => data.delete(key) };
  saveOAuthDestination('/beaverdeck/events?q=Warning');
  assert.equal(readOAuthDestination(true), '/beaverdeck/events?q=Warning');
  assert.equal(data.size, 0);
  global.sessionStorage = { getItem() { throw new Error('blocked'); }, setItem() { throw new Error('blocked'); } };
  assert.doesNotThrow(() => saveOAuthDestination('/beaverdeck/events'));
  assert.equal(readOAuthDestination(), null);
  delete global.window; delete global.document; delete global.sessionStorage;
});

test('external return metadata is scoped to the application prefix', () => {
  global.window = {};
  let prefix = '';
  global.document = { querySelector: () => ({ content: prefix }) };
  const data = new Map();
  global.sessionStorage = { setItem: (key, value) => data.set(key, value), getItem: (key) => data.get(key), removeItem: (key) => data.delete(key) };

  saveOAuthDestination('/events?q=one');
  prefix = '/beaverdeck';
  assert.equal(readOAuthDestination(), null);
  saveOAuthDestination('/beaverdeck/events?q=two');
  assert.equal(readOAuthDestination(true), '/beaverdeck/events?q=two');
  prefix = '';
  assert.equal(readOAuthDestination(true), '/events?q=one');

  delete global.window; delete global.document; delete global.sessionStorage;
});
