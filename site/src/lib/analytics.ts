import type { PostHog } from 'posthog-js';
import { versionFromPath } from './version.ts';

declare global {
  interface Window {
    __zarfDocsAnalytics?: Promise<void>;
  }
}

let client: PostHog | undefined;

/** Analytics must never interrupt a docs interaction, even if the SDK throws. */
export function captureDocsEvent(event: string, properties: Record<string, unknown> = {}) {
  try {
    client?.capture(event, {
      site: 'docs',
      docs_version: versionFromPath(window.location.pathname),
      ...properties,
    });
  } catch {
    // Analytics is optional; navigation, search, and copying must still work.
  }
}

function trackInteractions() {
  // Delegation also covers Pagefind's lazy UI and Astro client-side navigation.
  document.addEventListener('click', (event) => {
    if (!(event.target instanceof Element)) return;
    const copy = event.target.closest('.expressive-code .copy button');
    if (copy) {
      captureDocsEvent('docs_code_copy_clicked');
      return;
    }
    const result = event.target.closest<HTMLAnchorElement>('.pagefind-ui__result-link');
    if (result) {
      captureDocsEvent('docs_search_result_clicked', {
        destination_path: new URL(result.href, window.location.href).pathname,
      });
    }
  }, true);

  let searchTimer: ReturnType<typeof setTimeout> | undefined;
  const clearSearch = () => clearTimeout(searchTimer);
  document.addEventListener('input', (event) => {
    if (!(event.target instanceof HTMLInputElement) ||
        !event.target.matches('#starlight__search .pagefind-ui__search-input')) return;
    clearSearch();
    const length = event.target.value.trim().length;
    if (!length) return;
    searchTimer = setTimeout(() => {
      // Record usage without sending search text or copied code.
      captureDocsEvent('docs_search', { query_length: length });
    }, 600);
  });
  document.addEventListener('astro:before-swap', clearSearch);
}

/** Preview capture is opt-in and limited to this Netlify site's deploy hosts. */
export function shouldCaptureAnalytics(hostname: string, enableLocal = false, enablePreview = false): boolean {
  return hostname === 'docs.zarf.dev'
    || (enableLocal && ['localhost', '127.0.0.1', '[::1]'].includes(hostname))
    || (enablePreview && /^[a-z0-9][a-z0-9-]*--zarf-docs\.netlify\.app$/.test(hostname));
}

/** A single SDK instance owns both initial and history-change pageviews. */
export function initializeAnalytics(
  token: string | undefined,
  loadSDK = () => import('posthog-js'),
  enableLocal = false,
  enablePreview = false,
): Promise<void> {
  if (!token || !shouldCaptureAnalytics(window.location.hostname, enableLocal, enablePreview)) {
    return Promise.resolve();
  }
  if (window.__zarfDocsAnalytics) return window.__zarfDocsAnalytics;
  window.__zarfDocsAnalytics = (async () => {
    try {
      const { default: posthog } = await loadSDK();
      posthog.init(token, {
        api_host: 'https://f.defenseunicorns.com',
        ui_host: 'https://us.posthog.com',
        defaults: '2026-01-30',
        persistence: 'localStorage+cookie',
        cross_subdomain_cookie: true,
        capture_pageview: 'history_change',
        capture_pageleave: false,
        autocapture: false,
        disable_session_recording: true,
        capture_exceptions: false,
        capture_performance: false,
        enable_heatmaps: false,
        disable_surveys: true,
        loaded: (sdk) => sdk.register({ site: 'docs' }),
      });
      client = posthog;
      trackInteractions();
    } catch {
      // Missing/blocked SDK or unavailable storage must not block rendering.
    }
  })();
  return window.__zarfDocsAnalytics;
}
