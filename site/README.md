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
**Latest** at the root and each archived release lives under a `/<slug>/` subpath
(e.g. `/v0-76/`). It is one Astro build, not one per version.

For each release tag, `hack/build-versions.mjs` checks out a throwaway git
worktree and stages that tag's docs content into `src/content/docs/<slug>/`,
regenerating its examples and schema (`src/assets/schema/<slug>.json`) from the
tag's `examples/` and `zarf.schema.json`. Archived content therefore renders with
the **current** toolchain and components, never its own.

Two pieces make a single build serve many versions:

- **Sidebars** — `starlight-sidebar-topics` gives each version its own sidebar
  (Starlight otherwise has one global sidebar). `src/components/Sidebar.astro`
  overrides the plugin's sidebar to render only the scoped per-version sidebar,
  hiding the plugin's topic list. Version switching is the header
  `VersionSelect` picker (`src/components/VersionSelect.astro`), which keeps you
  on the same page in the chosen version when it exists.
- **Links & embeds** — `src/plugins/remark-link-rewrite.ts` runs only on pages
  under a version slug. It prefixes root-absolute links into known sections
  (`/commands/…` → `/v0-76/commands/…`) and adds one `../` to relative paths that
  escape the version subtree (shared assets, repo-root files, `examples/`),
  compensating for the extra nesting level.

Versions are discovered from GitHub Releases and reduced to the newest patch per
minor. Only the current major and the one before it are kept — the current major
shows its last 10 minors and the previous major its latest 3 minors.
The set is written to `versions.json`, read by `astro.config.ts` to build topics.

## 👀 Want to learn more?

Check out [Starlight's docs](https://starlight.astro.build/), read [the Astro documentation](https://docs.astro.build), or jump into the [Astro Discord server](https://astro.build/chat).
