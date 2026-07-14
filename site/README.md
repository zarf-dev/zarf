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
regenerating its examples and schema (`src/assets/schema/<slug>.json`) from the
tag's `examples/` and `zarf.schema.json`. Archived content therefore renders with
the **current** toolchain and components, never its own.

Versions are discovered from GitHub Releases and reduced to the newest patch per
minor. Only the current major and the one before it are kept — the current major shows
its last 10 minors and the previous major its latest 3 minors.
The set is written to versions.json, which the version switcher reads.

## 👀 Want to learn more?

Check out [Starlight's docs](https://starlight.astro.build/), read [the Astro documentation](https://docs.astro.build), or jump into the [Astro Discord server](https://astro.build/chat).
