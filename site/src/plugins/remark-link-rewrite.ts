// Remark plugin for archived docs staged under a version subtree
// (`src/content/docs/<slug>/`). It adjusts those pages only — Latest
// pages (no version slug) are left untouched:
//
//   1. Prefixes root-absolute internal links into known sections with the
//      version slug, e.g. `/commands/zarf` → `/v0-76/commands/zarf`.
//   2. Normalizes relative paths that escape the version subtree (shared assets
//      in `src/assets`, repo-root files, `examples/`). Nesting content one level
//      deeper shifts these up by one, so an escaping `../…` gains one `../`.
//   3. Migrates retired FAQ/schema references to the matching archived pages.
//      Shared public files keep their original root paths.
//
// Context is derived from `file.path` at render time, so it works for both the
// staged tree (versioned build) and the live tree (dev).

import { visit } from "unist-util-visit";
import path from "node:path";
import { statSync } from "node:fs";
import type { Root } from "mdast";
import type { VFile } from "vfile";

const VERSION_SLUG = /^v\d+-\d+$/;

// Maintained replacements for references retained in release tags.
const ARCHIVED_LINKS = new Map([
  ["/faq#what-is-the-zarf-agent", "/ref/init-package/#zarf-agent"],
  [
    "https://docs.zarf.dev/docs/create-a-zarf-package/zarf-schema#metadata",
    "/schema/v1alpha1-package/#ZarfMetadata",
  ],
  ["https://facebook.github.io/zstd/", "https://github.com/meta/zstd"],
  [
    "https://training.linuxfoundation.org/training/linux-tools-for-software-development-lfd108x/",
    "https://training.linuxfoundation.org/training/introduction-to-linux/",
  ],
  [
    "https://github.com/helm/helm/blob/main/pkg/releaseutil/manifest_sorter.go#L78",
    "https://github.com/helm/helm/blob/v3.19.2/pkg/releaseutil/manifest_sorter.go#L78",
  ],
  ["https://gitea.io/en-us/", "https://about.gitea.com/"],
  [
    "https://www.icann.org/en/board-activities-and-meetings/materials/approved-resolutions-special-meeting-of-the-icann-board-24-07-2024-en#section2.b",
    "https://www.icann.org/en/board-activities-and-meetings/materials/approved-resolutions-special-meeting-of-the-icann-board-29-07-2024-en#section2.b",
  ],
  ["https://www.gnu.org/software/make/", "https://savannah.gnu.org/projects/make/"],
  [
    "https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html",
    "https://tiswww.case.edu/php/chet/bash/bashref.html#The-Set-Builtin",
  ],
]);

interface Options {
  /** Absolute path to `src/content/docs/`. */
  srcDir: string;
  /** Top-level content sections eligible for prefixing (dir/page names). */
  sections: string[];
  /** Root of shared public assets, which must not receive a version prefix. */
  publicDir?: string;
  /** Release tags corresponding to the staged documentation subtrees. */
  versions?: { slug: string; ref: string }[];
}

/**
 * Prefix `url` with `prefix` when it is a root-absolute link into a known
 * section, migrating retired references first. Shared public files and other
 * external, protocol-relative, already-versioned, and non-section links stay unchanged.
 */
export function rewriteUrl(url: string, prefix: string, sections: Set<string>, publicDir?: string): string {
  url = ARCHIVED_LINKS.get(url) ?? url;
  if (!url.startsWith("/") || url.startsWith("//")) return url;
  const segment = url.slice(1).split(/[/#?]/, 1)[0];
  if (VERSION_SLUG.test(segment)) return url;
  if (!sections.has(segment)) return url;
  const pathname = url.split(/[?#]/, 1)[0];
  if (publicDir && statSync(path.join(publicDir, pathname), { throwIfNoEntry: false })?.isFile()) return url;
  return prefix + url;
}

/**
 * Add one `../` to a relative specifier when it resolves outside `versionRoot`,
 * compensating for the extra directory level of a staged version subtree.
 * Query/hash suffixes (e.g. `?raw`) are preserved.
 */
export function fixEscapingRelative(spec: string, fileDir: string, versionRoot: string): string {
  if (!spec.startsWith(".")) return spec;
  const target = path.resolve(fileDir, spec.split(/[?#]/, 1)[0]);
  const escapes = target !== versionRoot && !target.startsWith(versionRoot + path.sep);
  return escapes ? "../" + spec : spec;
}

export function rewriteVersionedAssetImport(spec: string, versionSlug: string): string | null {
  const match = spec.match(/^(?:\.\.\/)+(examples|packages)\/(.+\?raw)$/);
  if (!match) return null;

  const [, sourceKind, sourcePath] = match;
  return `/src/assets/versioned-assets/${versionSlug}/${sourceKind}/${sourcePath}`;
}

/** Archived package examples must reference the release that shipped them. */
export function rewriteArchivedSourceUrl(url: string, ref?: string): string {
  if (!ref) return url;
  return url.replace(
    /^https:\/\/github\.com\/(?:zarf-dev|defenseunicorns)\/zarf\/blob\/main\/(packages\/(?:gitea|zarf-registry)\/zarf\.yaml)([?#].*)?$/,
    (_match, file: string, suffix = "") => `https://github.com/zarf-dev/zarf/blob/${ref}/${file}${suffix}`,
  );
}

const SOURCE_NODES = new Set([
  "ImportDeclaration",
  "ImportExpression",
  "ExportAllDeclaration",
  "ExportNamedDeclaration",
]);

// Walk an ESTree, applying `fix` to the specifier of every static or dynamic
// import/export. MDX compiles from the ESTree, so this is the source of truth
// for `import x from "..."` and `import("...")` (e.g. <ExampleYAML src={...} />).
function fixEstreeSources(node: any, fix: (spec: string) => string): void {
  if (!node || typeof node !== "object") return;
  if (Array.isArray(node)) {
    for (const child of node) fixEstreeSources(child, fix);
    return;
  }
  const source = node.source;
  if (SOURCE_NODES.has(node.type) && source && typeof source.value === "string") {
    const fixed = fix(source.value);
    if (fixed !== source.value) {
      source.value = fixed;
      source.raw = JSON.stringify(fixed);
    }
  }
  for (const key of Object.keys(node)) {
    if (key !== "type") fixEstreeSources(node[key], fix);
  }
}

export function remarkLinkRewrite(options: Options) {
  const { srcDir } = options;
  const sections = new Set(options.sections);
  const versionRefs = new Map(options.versions?.map(({ slug, ref }) => [slug, ref]));

  return (tree: Root, file: VFile) => {
    if (!file.path) return;
    const rel = path.relative(srcDir, file.path);
    if (rel.startsWith("..")) return;

    const versionSlug = rel.split(path.sep)[0];
    if (!VERSION_SLUG.test(versionSlug)) return;
    const prefix = `/${versionSlug}`;
    const versionRoot = path.join(srcDir, versionSlug);
    const fileDir = path.dirname(file.path);

    const fixRelative = (spec: string) =>
      rewriteVersionedAssetImport(spec, versionSlug) ?? fixEscapingRelative(spec, fileDir, versionRoot);
    const fixUrl = (url: string) => {
      const sourceUrl = rewriteArchivedSourceUrl(url, versionRefs.get(versionSlug));
      const rewritten = rewriteUrl(sourceUrl, prefix, sections, options.publicDir);
      return rewritten.startsWith("/") ? rewritten : fixRelative(rewritten);
    };

    // Markdown links, link reference definitions, and images.
    visit(tree, ["link", "definition"], (node: any) => {
      node.url = fixUrl(node.url);
    });
    visit(tree, "image", (node: any) => {
      node.url = fixRelative(node.url);
    });

    // Raw HTML anchors/images embedded in Markdown.
    visit(tree, "html", (node: any) => {
      node.value = node.value.replace(
        /((?:href|src)=")([^"]*)/g,
        (_match: string, attr: string, url: string) => attr + fixUrl(url),
      );
    });

    // JSX props in MDX: string `href`/`src` (e.g. <LinkCard href="/ref/..." />)
    // and expression values carrying imports (e.g. <ExampleYAML src={import(...)} />).
    visit(tree, ["mdxJsxFlowElement", "mdxJsxTextElement"], (node: any) => {
      for (const attr of node.attributes ?? []) {
        if (attr.type !== "mdxJsxAttribute") continue;
        if ((attr.name === "href" || attr.name === "src") && typeof attr.value === "string") {
          attr.value = fixUrl(attr.value);
        } else if (attr.value?.data?.estree) {
          fixEstreeSources(attr.value.data.estree, fixRelative);
        }
      }
    });

    // Static/dynamic imports in ESM blocks and `{…}` expressions. MDX compiles
    // from the ESTree, not the node's source text, so mutate that.
    visit(tree, ["mdxjsEsm", "mdxFlowExpression", "mdxTextExpression"], (node: any) => {
      if (node.data?.estree) fixEstreeSources(node.data.estree, fixRelative);
    });
  };
}
