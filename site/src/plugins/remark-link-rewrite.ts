// Remark plugin for archived docs staged under a version subtree
// (`src/content/docs/<slug>/`). It does two things for those pages only — Latest
// pages (no version slug) are left untouched:
//
//   1. Prefixes root-absolute internal links into known sections with the
//      version slug, e.g. `/commands/zarf` → `/v0-76/commands/zarf`.
//   2. Normalizes relative paths that escape the version subtree (shared assets
//      in `src/assets`, repo-root files, `examples/`). Nesting content one level
//      deeper shifts these up by one, so an escaping `../…` gains one `../`.
//
// Context is derived from `file.path` at render time, so it works for both the
// staged tree (versioned build) and the live tree (dev).

import { visit } from "unist-util-visit";
import path from "node:path";
import type { Root } from "mdast";
import type { VFile } from "vfile";

const VERSION_SLUG = /^v\d+-\d+$/;

interface Options {
  /** Absolute path to `src/content/docs/`. */
  srcDir: string;
  /** Top-level content sections eligible for prefixing (dir/page names). */
  sections: string[];
}

/**
 * Prefix `url` with `prefix` when it is a root-absolute link into a known
 * section. Leaves external, protocol-relative, already-versioned, and
 * non-section links unchanged.
 */
export function rewriteUrl(url: string, prefix: string, sections: Set<string>): string {
  if (!url.startsWith("/") || url.startsWith("//")) return url;
  const segment = url.slice(1).split(/[/#?]/, 1)[0];
  if (VERSION_SLUG.test(segment)) return url;
  if (!sections.has(segment)) return url;
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

  return (tree: Root, file: VFile) => {
    if (!file.path) return;
    const rel = path.relative(srcDir, file.path);
    if (rel.startsWith("..")) return;

    const versionSlug = rel.split(path.sep)[0];
    if (!VERSION_SLUG.test(versionSlug)) return;
    const prefix = `/${versionSlug}`;
    const versionRoot = path.join(srcDir, versionSlug);
    const fileDir = path.dirname(file.path);

    const fixRelative = (spec: string) => fixEscapingRelative(spec, fileDir, versionRoot);
    const fixUrl = (url: string) => (url.startsWith("/") ? rewriteUrl(url, prefix, sections) : fixRelative(url));

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
