import { test } from 'node:test';
import assert from 'node:assert/strict';
import type { PostHog } from 'posthog-js';
import { initializeAnalytics, captureDocsEvent, shouldCaptureAnalytics } from './analytics.ts';

test('analytics is optional, initializes once, and cannot break interactions', async (t) => {
  const originalWindow = Object.getOwnPropertyDescriptor(globalThis, 'window');
  const originalDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
  const listeners: string[] = [];
  const fakeWindow = { location: { hostname: 'localhost', pathname: '/v0-76/ref/' } };
  Object.defineProperty(globalThis, 'window', { configurable: true, value: fakeWindow });
  Object.defineProperty(globalThis, 'document', {
    configurable: true, value: { addEventListener: (name: string) => listeners.push(name) },
  });
  t.after(() => {
    if (originalWindow) Object.defineProperty(globalThis, 'window', originalWindow);
    else Reflect.deleteProperty(globalThis, 'window');
    if (originalDocument) Object.defineProperty(globalThis, 'document', originalDocument);
    else Reflect.deleteProperty(globalThis, 'document');
  });
  let loads = 0;
  let inits = 0;
  let config: Record<string, any> = {};
  const captures: unknown[][] = [];
  const sdk = {
    init: (_token: string, options: Record<string, any>) => { inits++; config = options; },
    capture: (...args: unknown[]) => captures.push(args),
  } as unknown as PostHog;
  const load = async () => { loads++; return { default: sdk } as typeof import('posthog-js'); };

  await initializeAnalytics('public-test-token', load);
  assert.equal(loads, 0, 'local and preview hosts must not load analytics');
  fakeWindow.location.hostname = 'preview.example.com';
  await initializeAnalytics('public-test-token', load, true);
  assert.equal(loads, 0, 'local opt-in must not enable preview deployments');
  fakeWindow.location.hostname = 'localhost';
  await initializeAnalytics(undefined, load, true);
  assert.equal(loads, 0, 'missing configuration must not load analytics');
  assert.doesNotThrow(() => captureDocsEvent('before_init'));

  await Promise.all([
    initializeAnalytics('public-test-token', load, true),
    initializeAnalytics('public-test-token', load, true),
  ]);
  assert.equal(loads, 1);
  assert.equal(inits, 1);
  assert.equal(config.api_host, 'https://f.defenseunicorns.com');
  assert.equal(config.ui_host, 'https://us.posthog.com');
  assert.equal(config.cross_subdomain_cookie, true);
  assert.equal(config.persistence, 'localStorage+cookie');
  assert.equal(config.capture_pageview, 'history_change');
  assert.equal(captures.length, 0, 'manual captures must not duplicate SDK pageviews');
  assert.equal(listeners.filter((name) => name === 'click').length, 1);
  captureDocsEvent('documentation_version_selected', { target_version: 'current' });
  assert.deepEqual(captures[0], ['documentation_version_selected', {
    site: 'docs', docs_version: 'v0-76', target_version: 'current',
  }]);
  sdk.capture = () => { throw new Error('storage unavailable'); };
  assert.doesNotThrow(() => captureDocsEvent('docs_code_copy_clicked'));

  fakeWindow.location.hostname = 'docs.zarf.dev';
  delete window.__zarfDocsAnalytics;
  await assert.doesNotReject(initializeAnalytics('public-test-token', async () => {
    throw new Error('SDK blocked');
  }));
  delete window.__zarfDocsAnalytics;
  await assert.doesNotReject(initializeAnalytics('public-test-token', async () => ({
    default: { init: () => { throw new Error('initialization failed'); } },
  }) as unknown as typeof import('posthog-js')));
});


test('production, local, and docs preview hosts have separate capture policies', () => {
  assert.equal(shouldCaptureAnalytics('docs.zarf.dev'), true);
  for (const host of ['localhost', '127.0.0.1', '[::1]']) {
    assert.equal(shouldCaptureAnalytics(host), false);
    assert.equal(shouldCaptureAnalytics(host, true), true);
    assert.equal(shouldCaptureAnalytics(host, false, true), false);
  }
  for (const host of ['deploy-preview-5529--zarf-docs.netlify.app', 'analytics-fix--zarf-docs.netlify.app']) {
    assert.equal(shouldCaptureAnalytics(host), false);
    assert.equal(shouldCaptureAnalytics(host, true), false);
    assert.equal(shouldCaptureAnalytics(host, false, true), true);
  }
  for (const host of ['zarf.dev', 'example.com', 'deploy-preview-1--other-site.netlify.app',
    'deploy-preview-1--zarf-docs.netlify.app.example.com', 'zarf-docs.netlify.app']) {
    assert.equal(shouldCaptureAnalytics(host, true, true), false);
  }
});
