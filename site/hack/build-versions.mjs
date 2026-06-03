// Builds the current checkout at the site root ("Latest") and a window of
// archived releases into `/<slug>/` subpaths. See site/README.md for the strategy.

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

// A tag's docs content, kept from its worktree; everything else under `site/`
// comes from the current checkout.
const docsPaths = ["src/content/docs"];

// Entries never copied from the current checkout (`node_modules` is symlinked).
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

function aboveFloor(minor) {
  const [maj = 0, min = 0] = parseSemver(minor);
  const [fMaj = 0, fMin = 0] = parseSemver(MIN_VERSION);
  return maj > fMaj || (maj === fMaj && min >= fMin);
}

function toVersion(tag) {
  const minor = minorKey(tag);
  return { ref: tag, label: minor, slug: slugOf(minor) };
}

// Released minors down to MIN_VERSION, newest first.
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
  const archived = minorsDesc.filter(aboveFloor).map((m) => toVersion(newestByMinor.get(m)));
  return { latest: minorsDesc[0], archived };
}

// ---------------------------------------------------------------------------
// Build steps
// ---------------------------------------------------------------------------

// Replace the worktree's `site/` with the current checkout's, keeping the tag's docs.
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
      // The tag's docs are restored from the stash below, not copied here.
      filter: (src) => !skipAbs.some((s) => under(src, s)) && !dataAbs.some((d) => under(src, d)),
    });
    for (const rel of docsPaths) {
      await fs.cp(path.join(stash, rel), path.join(worktreeSite, rel), { recursive: true });
    }
  } finally {
    await fs.rm(stash, { recursive: true, force: true });
  }
  await fs.symlink(path.join(siteDir, "node_modules"), path.join(worktreeSite, "node_modules"), "dir");
}

async function rewriteVersionLinks(dir, slug) {
  // Astro's `base` doesn't rewrite root-absolute links hardcoded in Markdown, so
  // prefix href/src values that aren't already under /<slug>/ or protocol-relative.
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
    // Regenerate this tag's schema and examples with the overlaid (current) scripts.
    npm(["run", "prebuild"], worktreeSite);
    npm(["exec", "--", "astro", "build", "--base", `/${slug}`], worktreeSite);
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

  // Written before any build so every build's switcher shows the same options.
  await fs.writeFile(
    path.join(siteDir, "versions.json"),
    JSON.stringify({ versions: archived.map(({ ref, label, slug }) => ({ ref, label, slug })) }, null, 2) + "\n",
  );

  // Build Latest at the root. `astro check` runs separately in CI (`npm run check`).
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
