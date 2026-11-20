import { test } from 'node:test';
import assert from 'node:assert/strict';
import { appBasePath, withBasePath } from './paths.js';

test('runtime prefix is independent of the active route', () => {
  global.window = { location: { pathname: '/beaverdeck/events/nested' } };
  for (const prefix of ['', '/beaverdeck']) {
    global.document = { querySelector: () => ({ content: prefix }) };
    assert.equal(appBasePath(), prefix);
    for (const path of ['/api/me', '/api/auth/login', '/api/auth/google/start', '/api/auth/oidc/start', '/api/auth/entra/start', '/api/auth/logout']) {
      assert.equal(withBasePath(path), prefix + path);
    }
  }
  delete global.window;
  delete global.document;
});
