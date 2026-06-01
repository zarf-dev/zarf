// Builds the documentation site for every version listed in versions.json.
//
// The current docs (this checkout) are built at the site root as "Latest".
// Each archived version is built from its git tag into a `/<slug>/` subpath,
// so no version content is duplicated in the repository.
//
// For an archived build we check the tag out into a throwaway git worktree,
// overlay the current toolchain bits needed for versioning (astro config,
// switcher components, manifest), build it with `base` set to the version slug,
// then rewrite the absolute links that Astro's `base` leaves untouched (links
// hardcoded in Markdown bodies, e.g. `/commands/...`). The resulting `dist/` is
// copied under the top-level `dist/<slug>/`.

import { execFileSync } from "node:child_process";
import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const siteDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repoDir = path.resolve(siteDir, "..");
const distDir = path.join(siteDir, "dist");
const worktreeRoot = path.join(repoDir, ".docs-version-builds");

// Toolchain files copied from the current checkout onto each version's worktree
// so archived builds get the base-aware config and the version switcher.
const overlay = [
  "astro.config.ts",
  "versions.json",
  "src/components/SkipLink.astro",
  "src/components/Sidebar.astro",
  "src/components/VersionSelect.astro",
];

function git(args, opts = {}) {
  return execFileSync("git", args, { cwd: repoDir, stdio: "inherit", ...opts });
}

function npm(args, cwd, env) {
  execFileSync("npm", args, { cwd, stdio: "inherit", env: { ...process.env, ...env } });
}

// Reuse the current node_modules when the version's lockfile is identical,
// avoiding a full `npm ci` per version. Falls back to `npm ci` if it diverges.
async function installDeps(worktreeSite) {
  const read = (p) => fs.readFile(p, "utf8").catch(() => null);
  const [rootLock, wtLock] = await Promise.all([
    read(path.join(siteDir, "package-lock.json")),
    read(path.join(worktreeSite, "package-lock.json")),
  ]);
  if (rootLock && rootLock === wtLock) {
    await fs.symlink(path.join(siteDir, "node_modules"), path.join(worktreeSite, "node_modules"), "dir");
  } else {
    npm(["ci"], worktreeSite);
  }
}

async function rewriteVersionLinks(dir, slug) {
  // Prefix root-absolute links that aren't already under /<slug>/ and aren't
  // protocol-relative (//host). Scoped to href=/src= attribute values.
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

async function buildVersion(version) {
  const { slug, ref } = version;
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
    for (const file of overlay) {
      await fs.copyFile(path.join(siteDir, file), path.join(worktreeSite, file));
    }
    await installDeps(worktreeSite);
    npm(["run", "prebuild"], worktreeSite);
    npm(["exec", "astro", "build"], worktreeSite, { DOCS_BASE: `/${slug}` });
    await rewriteVersionLinks(path.join(worktreeSite, "dist"), slug);
    await fs.cp(path.join(worktreeSite, "dist"), path.join(distDir, slug), { recursive: true });
  } finally {
    git(["worktree", "remove", "--force", worktree]);
  }
}

async function main() {
  const manifest = JSON.parse(await fs.readFile(path.join(siteDir, "versions.json"), "utf8"));

  // Build the current docs at the root. `astro check` is skipped here (it runs
  // in PR CI via `npm run build`); the deploy only needs the build output.
  await fs.rm(distDir, { recursive: true, force: true });
  npm(["run", "prebuild"], siteDir);
  npm(["exec", "astro", "build"], siteDir);

  if (manifest.versions.length > 0) {
    await fs.rm(worktreeRoot, { recursive: true, force: true });
    for (const version of manifest.versions) {
      await buildVersion(version);
    }
    await fs.rm(worktreeRoot, { recursive: true, force: true });
  }
}

await main();
