// Builds the documentation site for the current checkout plus a window of
// archived releases, using a content-snapshot strategy.
//
// The current docs (this checkout) are built at the site root as "Latest".
// Each archived version is built from its release tag into a `/<slug>/` subpath.
//
// archived versions are rendered with the current toolchain, never
// their own. For each version we check the tag out into a throwaway git
// worktree, then replace its `site/` wholesale with the tag's docs.
// The tag's repo-level data — `examples/` and `zarf.schema.json` — is left in place at the
// worktree root and consumed by `prebuild`. The current `node_modules` is reused.
//
// Versions are discovered from GitHub Releases, deduplicated to the newest
// patch per minor, and floored at MIN_VERSION.

import { execFileSync } from "node:child_process";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const siteDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repoDir = path.resolve(siteDir, "..");
const distDir = path.join(siteDir, "dist");
const worktreeRoot = path.join(repoDir, ".docs-version-builds");

const REPO = "zarf-dev/zarf";
// Inclusive floor: archived versions older than this minor are not built.
const MIN_VERSION = "v0.76";

// Paths under `site/` that hold an archived version's *data* and are kept from
// the tag's worktree. Everything else under `site/` is toolchain, replaced with
// the current checkout.
const docsPaths = ["src/content/docs"];

// Top-level `site/` entries never copied from the current checkout: installed
// deps and build artifacts. `node_modules` is symlinked separately.
const overlaySkip = new Set(["node_modules", "dist", ".astro"]);

function git(args, opts = {}) {
  return execFileSync("git", args, { cwd: repoDir, stdio: "inherit", ...opts });
}

function npm(args, cwd, env) {
  execFileSync("npm", args, { cwd, stdio: "inherit", env: { ...process.env, ...env } });
}

// ---------------------------------------------------------------------------
// Version discovery
// ---------------------------------------------------------------------------

const parseSemver = (tag) => tag.replace(/^v/, "").split(".").map(Number);
const minorKey = (tag) => tag.replace(/\.\d+$/, "");
const slugOf = (minor) => minor.replace(/\./g, "-");

function cmpMinorDesc(a, b) {
  const [aMaj = 0, aMin = 0] = parseSemver(a);
  const [bMaj = 0, bMin = 0] = parseSemver(b);
  return bMaj - aMaj || bMin - aMin;
}

// True when `minor` is >= the configured floor.
function aboveFloor(minor) {
  const [maj = 0, min = 0] = parseSemver(minor);
  const [fMaj = 0, fMin = 0] = parseSemver(MIN_VERSION);
  return maj > fMaj || (maj === fMaj && min >= fMin);
}

// Returns { ref, label, slug } for a full tag like "v0.76.3".
function toVersion(tag) {
  const minor = minorKey(tag);
  return { ref: tag, label: minor, slug: slugOf(minor) };
}

// Discover every released minor down to MIN_VERSION, each pinned to its own
// `/<slug>/` subpath. Returns { archived: [{ ref, label, slug }] } sorted
// newest-first.
async function discoverVersions() {
  const headers = { Accept: "application/vnd.github+json" };
  if (process.env.GITHUB_TOKEN) headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;
  const res = await fetch(`https://api.github.com/repos/${REPO}/releases?per_page=100`, { headers });
  if (!res.ok) {
    throw new Error(`GitHub API returned ${res.status} ${res.statusText} for ${REPO} releases`);
  }
  const releases = await res.json();
  const tags = releases.filter((r) => !r.prerelease && !r.draft).map((r) => r.tag_name);

  // Keep only the newest patch per minor.
  const newestByMinor = new Map();
  for (const tag of tags) {
    if (!/^v?\d+\.\d+\.\d+$/.test(tag)) continue;
    const minor = minorKey(tag);
    const current = newestByMinor.get(minor);
    if (!current || parseSemver(tag)[2] > parseSemver(current)[2]) {
      newestByMinor.set(minor, tag);
    }
  }

  const minorsDesc = [...newestByMinor.keys()].sort(cmpMinorDesc);
  // Every released minor down to the floor gets a pinned subpath, newest included.
  const archived = minorsDesc.filter(aboveFloor).map((m) => toVersion(newestByMinor.get(m)));
  return { latest: minorsDesc[0], archived };
}

// ---------------------------------------------------------------------------
// Build steps
// ---------------------------------------------------------------------------

// Replace the worktree's `site/` with the current checkout's, keeping only the
// tag's data.
async function overlayToolchain(worktreeSite) {
  const skipAbs = [...overlaySkip].map((d) => path.join(siteDir, d));
  const dataAbs = docsPaths.map((d) => path.join(siteDir, d));
  const under = (p, root) => p === root || p.startsWith(root + path.sep);

  // Preserve the tag's data across the wholesale replacement of `site/`.
  const stash = await fs.mkdtemp(path.join(os.tmpdir(), "zarf-docs-data-"));
  try {
    for (const rel of docsPaths) {
      await fs.cp(path.join(worktreeSite, rel), path.join(stash, rel), { recursive: true });
    }
    await fs.rm(worktreeSite, { recursive: true, force: true });
    await fs.cp(siteDir, worktreeSite, {
      recursive: true,
      // Skip installed deps, build artifacts, and the current checkout's data —
      // the latter is restored from the tag below.
      filter: (src) => !skipAbs.some((s) => under(src, s)) && !dataAbs.some((d) => under(src, d)),
    });
    for (const rel of docsPaths) {
      await fs.cp(path.join(stash, rel), path.join(worktreeSite, rel), { recursive: true });
    }
  } finally {
    await fs.rm(stash, { recursive: true, force: true });
  }
  // Reuse the current install — never resolve the tag's own dependencies.
  await fs.symlink(path.join(siteDir, "node_modules"), path.join(worktreeSite, "node_modules"), "dir");
}

async function rewriteVersionLinks(dir, slug) {
  // Prefix root-absolute links that aren't already under /<slug>/ and aren't
  // protocol-relative (//host). Scoped to href=/src= attribute values, since
  // Astro's `base` doesn't rewrite links hardcoded in Markdown bodies.
  const escaped = slug.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const re = new RegExp(`(\\s(?:href|src)=")/(?!${escaped}/)(?!/)`, "g");
  const entries = await fs.readdir(dir, { withFileTypes: true });
  for (const entry of entries) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      await rewriteVersionLinks(p, slug);
    } else if (entry.name.endsWith(".html")) {
      const html = await fs.readFile(p, "utf8");
      await fs.writeFile(p, html.replace(re, `$1/${slug}/`));
    }
  }
}

async function buildVersion({ ref, slug }) {
  const worktree = path.join(worktreeRoot, slug);
  const worktreeSite = path.join(worktree, "site");

  console.log(`\n=== Building ${slug} from ${ref} ===`);
  // CI often uses a shallow clone without tag commits; fetch the tag's commit.
  try {
    git(["fetch", "--depth=1", "origin", "tag", ref, "--no-tags"]);
  } catch {
    console.warn(`git fetch of tag ${ref} failed; assuming it is already present`);
  }
  git(["worktree", "add", "--detach", worktree, ref]);
  try {
    await overlayToolchain(worktreeSite);
    // prebuild regenerates this tag's schema and examples from its own data,
    // using the overlaid (current) scripts.
    npm(["run", "prebuild"], worktreeSite);
    npm(["exec", "astro", "build"], worktreeSite, { DOCS_BASE: `/${slug}` });
    await rewriteVersionLinks(path.join(worktreeSite, "dist"), slug);
    await fs.cp(path.join(worktreeSite, "dist"), path.join(distDir, slug), { recursive: true });
  } finally {
    git(["worktree", "remove", "--force", worktree]);
  }
}

async function main() {
  const { latest, archived } = await discoverVersions();
  console.log(`Latest (root, tracks current checkout): ${latest ?? "(unknown)"}`);
  console.log(`Pinned versions (>= ${MIN_VERSION}): ${archived.map((v) => v.ref).join(", ") || "(none)"}`);

  // The version switcher reads this manifest; write it before any build so the
  // root and every archived build render the same set of options.
  await fs.writeFile(
    path.join(siteDir, "versions.json"),
    JSON.stringify({ versions: archived.map(({ ref, label, slug }) => ({ ref, label, slug })) }, null, 2) + "\n",
  );

  // Build the current docs at the root. `astro check` is skipped here (it runs
  // in PR CI via `npm run build`); the deploy only needs the build output.
  await fs.rm(distDir, { recursive: true, force: true });
  npm(["run", "prebuild"], siteDir);
  npm(["exec", "astro", "build"], siteDir);

  if (archived.length > 0) {
    await fs.rm(worktreeRoot, { recursive: true, force: true });
    for (const version of archived) {
      await buildVersion(version);
    }
    await fs.rm(worktreeRoot, { recursive: true, force: true });
  }

  console.log("\nVersioned documentation build complete.");
}

await main();
