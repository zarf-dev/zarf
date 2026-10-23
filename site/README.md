# Zarf - Site

## 🚀 Project Structure

Inside of your Astro + Starlight project, you'll see the following folders and files:

```plaintext
.
├── public/
├── src/
│   ├── assets/
│   ├── content/
│   │   ├── docs/
│   │   └── config.ts
│   └── env.d.ts
├── astro.config.mjs
├── package.json
└── tsconfig.json
```

Starlight looks for `.md` or `.mdx` files in the `src/content/docs/` directory. Each file is exposed as a route based on its file name.

Images can be added to `src/assets/` and embedded in Markdown with a relative link.

Static assets, like favicons, can be placed in the `public/` directory.

## 🧞 Commands

All commands are run from the root of the project, from a terminal:

| Command                   | Action                                           |
| :------------------------ | :----------------------------------------------- |
| `npm install`             | Installs dependencies                            |
| `npm run dev`             | Starts local dev server at `localhost:4321`      |
| `npm run build`           | Build your production site to `./dist/`          |
| `npm run preview`         | Preview your build locally, before deploying     |
| `npm run astro ...`       | Run CLI commands like `astro add`, `astro check` |
| `npm run astro -- --help` | Get help using the Astro CLI                     |
| `npm run build:versions`  | Build Latest plus archived versions (see below)  |

## Serving Multiple Versions

`npm run build:versions` produces a single site where the current checkout is
**Latest** at the root and each versioned release lives under a `/<slug>/` subpath
(e.g. `/v0-76/`).

For each release tag, `hack/build-versions.mjs` checks out a throwaway git
worktree and stages that tag's docs content into `src/content/docs/<slug>/`,
regenerating its examples and schema under
`src/assets/versioned-assets/<slug>/` from the tag's `examples/` and
`zarf.schema.json`. Archived content therefore renders with the **current**
toolchain and components, never its own.

Versions are discovered from git tags and reduced to the newest patch per
minor. Only the current major and the one before it are kept — the current major shows
its last 10 minors and the previous major its latest 3 minors.
The set is written to versions.json, which the version switcher reads.

## 👀 Want to learn more?

Check out [Starlight's docs](https://starlight.astro.build/), read [the Astro documentation](https://docs.astro.build), or jump into the [Astro Discord server](https://astro.build/chat).

## Analytics

This static Astro site uses PostHog project **Zarf (683708)**. Set
`PUBLIC_POSTHOG_PROJECT_TOKEN` to the existing project's public token in the
build environment (or a gitignored `site/.env` for a local production build).
The value is intentionally public in the browser bundle. Never use a personal
API key here. An unset token disables analytics without affecting rendering.
By default only `docs.zarf.dev` captures events. Local testing requires an
explicit opt-in; preview deployment hosts remain excluded.
Rebuild the site after changing the build environment.

The ingestion host is `https://f.defenseunicorns.com` and the UI host is
`https://us.posthog.com`. `localStorage+cookie` persistence and a `.zarf.dev`
cookie preserve anonymous identity across sibling subdomains using the same
project token and cookie configuration. The SDK initializes once and owns
initial and history-change `$pageview` capture, including Astro navigation.

Custom events:

| Event | Meaning | Properties |
| --- | --- | --- |
| `docs_search` | Nonempty search input settled for 600 ms | `query_length` |
| `docs_search_result_clicked` | A Pagefind result was selected | `destination_path` |
| `docs_code_copy_clicked` | A code-copy button was clicked (does not assert clipboard success) | none |
| `documentation_version_selected` | A docs version was selected | `target_version` |

Custom events include `site: docs` and `docs_version`. Search text and copied
code are not sent. Autocapture, replay, surveys, and exception capture are
explicitly disabled. SDK loading and capture failures are ignored so docs
remain usable. Existing Google analytics is preserved.

Use the existing [Analytics basics dashboard](https://us.posthog.com/project/683708/dashboard/2251382)
for reporting, filtering docs charts to `$host = docs.zarf.dev`.

Public integration sources: [PostHog Astro](https://posthog.com/docs/libraries/astro),
[SDK configuration](https://posthog.com/docs/libraries/js/config), and
[cookie persistence](https://posthog.com/docs/libraries/js/persistence).
This setup used `@posthog/wizard` 2.87.0 and `posthog-js` 1.438.21
([public npm package](https://www.npmjs.com/package/posthog-js/v/1.438.21)).

### Test analytics locally

Keep the existing project token in your gitignored `.env`. To test the full
search UI, use a production build and preview (Pagefind search is unavailable
in the Astro development server):

```sh
cd ~/Developer/zarf/site
PUBLIC_POSTHOG_ENABLE_LOCAL=true npm run build
npm run preview -- --host 127.0.0.1
```

Open the preview URL printed by Astro, visit a page, click a code-copy button,
and search for a docs topic. In the Zarf project's Activity view, inspect
`$pageview`, `docs_code_copy_clicked`, `docs_search`, and
`docs_search_result_clicked`, filtering `$host` to your local hostname.
These are real events sent to the existing Zarf project through the proxy.
The production dashboard excludes localhost, so local test events do not
appear in its charts. Allow a few seconds for batched events to arrive.
Localhost cookies cannot verify identity sharing across production subdomains.

For development without search, run
`PUBLIC_POSTHOG_ENABLE_LOCAL=true npm run dev`. After testing, rebuild without
the flag to restore the default host restriction. The flag can also be set in
your gitignored `.env`; remove it or set it to `false` when finished.
