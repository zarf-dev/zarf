import { test } from "node:test";
import assert from "node:assert/strict";
import path from "node:path";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { rewriteUrl, fixEscapingRelative, rewriteVersionedAssetImport, rewriteArchivedSourceUrl, remarkLinkRewrite } from "./remark-link-rewrite.ts";

const sections = new Set(["commands", "ref", "tutorials", "getting-started", "faq", "schema"]);
const prefix = "/v0-76";

test("prefixes root-absolute links into known sections", () => {
  assert.equal(rewriteUrl("/commands/zarf", prefix, sections), "/v0-76/commands/zarf");
  assert.equal(rewriteUrl("/ref/examples/argocd/", prefix, sections), "/v0-76/ref/examples/argocd/");
});

test("prefixes single-page sections with and without trailing slash", () => {
  assert.equal(rewriteUrl("/faq", prefix, sections), "/v0-76/faq");
  assert.equal(rewriteUrl("/faq/", prefix, sections), "/v0-76/faq/");
});

test("preserves anchors and queries", () => {
  assert.equal(rewriteUrl("/ref/actions/#ondeploy", prefix, sections), "/v0-76/ref/actions/#ondeploy");
});

test("leaves unknown sections untouched", () => {
  assert.equal(rewriteUrl("/llms.txt", prefix, sections), "/llms.txt");
  assert.equal(rewriteUrl("/architecture", prefix, sections), "/architecture");
});

test("ignores external, protocol-relative, and relative links", () => {
  assert.equal(rewriteUrl("https://x.io/commands", prefix, sections), "https://x.io/commands");
  assert.equal(rewriteUrl("//cdn/commands", prefix, sections), "//cdn/commands");
  assert.equal(rewriteUrl("../commands/zarf", prefix, sections), "../commands/zarf");
});

test("does not double-prefix already-versioned links", () => {
  assert.equal(rewriteUrl("/v0-75/commands/zarf", prefix, sections), "/v0-75/commands/zarf");
});

test("keeps shared public recordings at their root path while versioning documentation", (t) => {
  const publicDir = mkdtempSync(path.join(tmpdir(), "zarf-public-"));
  t.after(() => rmSync(publicDir, { recursive: true, force: true }));
  mkdirSync(path.join(publicDir, "tutorials"));
  writeFileSync(path.join(publicDir, "tutorials/recording.html"), "recording");
  assert.equal(rewriteUrl("/tutorials/recording.html?theme=dark#frame", prefix, sections, publicDir),
    "/tutorials/recording.html?theme=dark#frame");
  assert.equal(rewriteUrl("/tutorials/", prefix, sections, publicDir), "/v0-76/tutorials/");
  assert.equal(rewriteUrl("/tutorials/guide", prefix, sections, publicDir), "/v0-76/tutorials/guide");
});

test("resolves retired references to the matching archived reference pages", () => {
  assert.equal(rewriteUrl("/faq#what-is-the-zarf-agent", prefix, sections), "/v0-76/ref/init-package/#zarf-agent");
  assert.equal(rewriteUrl("https://docs.zarf.dev/docs/create-a-zarf-package/zarf-schema#metadata", prefix, sections),
    "/v0-76/schema/v1alpha1-package/#ZarfMetadata");
  assert.equal(rewriteUrl("/faq#another-question", prefix, sections), "/v0-76/faq#another-question");
});

test("pins archived package source links to their release tag", () => {
  assert.equal(rewriteArchivedSourceUrl("https://github.com/zarf-dev/zarf/blob/main/packages/gitea/zarf.yaml", "v0.80.0"),
    "https://github.com/zarf-dev/zarf/blob/v0.80.0/packages/gitea/zarf.yaml");
  assert.equal(rewriteArchivedSourceUrl("https://github.com/defenseunicorns/zarf/blob/main/packages/zarf-registry/zarf.yaml#L1", "v0.81.1"),
    "https://github.com/zarf-dev/zarf/blob/v0.81.1/packages/zarf-registry/zarf.yaml#L1");
  const otherSource = "https://github.com/zarf-dev/zarf/blob/main/src/main.go";
  assert.equal(rewriteArchivedSourceUrl(otherSource, "v0.80.0"), otherSource);
  const packageSource = "https://github.com/zarf-dev/zarf/blob/main/packages/gitea/zarf.yaml";
  assert.equal(rewriteArchivedSourceUrl(packageSource), packageSource);
});

test("rewrites archived Markdown, HTML and JSX links without changing Latest pages", (t) => {
  const root = mkdtempSync(path.join(tmpdir(), "zarf-remark-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const publicDir = path.join(root, "public");
  mkdirSync(path.join(publicDir, "tutorials"), { recursive: true });
  writeFileSync(path.join(publicDir, "tutorials/recording.html"), "recording");
  const srcDir = path.join(root, "docs");
  const transform = remarkLinkRewrite({ srcDir, sections: [...sections], publicDir,
    versions: [{ slug: "v0-76", ref: "v0.76.4" }] });
  const tree = () => ({ type: "root", children: [
    { type: "link", url: "https://docs.zarf.dev/docs/create-a-zarf-package/zarf-schema#metadata", children: [] },
    { type: "link", url: "/faq#what-is-the-zarf-agent", children: [] },
    { type: "html", value: '<iframe src="/tutorials/recording.html"></iframe>' },
    { type: "mdxJsxFlowElement", name: "iframe", attributes: [
      { type: "mdxJsxAttribute", name: "src", value: "/tutorials/recording.html" },
    ], children: [] },
    { type: "link", url: "https://github.com/zarf-dev/zarf/blob/main/packages/gitea/zarf.yaml", children: [] },
    { type: "link", url: "https://gitea.io/en-us/", children: [] },
    { type: "html", value: '<a href="https://www.gnu.org/software/make/">make</a>' },
    { type: "mdxJsxFlowElement", name: "a", attributes: [
      { type: "mdxJsxAttribute", name: "href", value: "https://facebook.github.io/zstd/" },
    ], children: [] },
  ] });
  const archived = tree();
  transform(archived as any, { path: path.join(srcDir, "v0-76/page.mdx") } as any);
  assert.equal(archived.children[0].url, "/v0-76/schema/v1alpha1-package/#ZarfMetadata");
  assert.equal(archived.children[1].url, "/v0-76/ref/init-package/#zarf-agent");
  assert.equal(archived.children[2].value, '<iframe src="/tutorials/recording.html"></iframe>');
  assert.equal(archived.children[3].attributes?.[0].value, "/tutorials/recording.html");
  assert.equal(archived.children[4].url, "https://github.com/zarf-dev/zarf/blob/v0.76.4/packages/gitea/zarf.yaml");
  assert.equal(archived.children[5].url, "https://about.gitea.com/");
  assert.equal(archived.children[6].value, '<a href="https://savannah.gnu.org/projects/make/">make</a>');
  assert.equal(archived.children[7].attributes?.[0].value, "https://github.com/meta/zstd");
  const latest = tree();
  transform(latest as any, { path: path.join(srcDir, "page.mdx") } as any);
  assert.deepEqual(latest, tree());
});

// fixEscapingRelative: docs root /docs, version subtree /docs/v0-76
const srcDir = "/docs";
const versionRoot = path.join(srcDir, "v0-76");

test("adds one ../ to asset paths escaping the version subtree", () => {
  // image in /docs/v0-76/index.mdx pointing at shared src/assets
  const fileDir = path.join(versionRoot);
  assert.equal(fixEscapingRelative("../../assets/x.svg", fileDir, versionRoot), "../../../assets/x.svg");
});

test("adds one ../ to generic escaping imports, preserving query", () => {
  const fileDir = path.join(versionRoot, "ref");
  assert.equal(
    fixEscapingRelative("../../../../../examples/config-file/zarf.yaml?raw", fileDir, versionRoot),
    "../../../../../../examples/config-file/zarf.yaml?raw",
  );
});

test("maps versioned asset repo imports to staged assets", () => {
  assert.equal(
    rewriteVersionedAssetImport("../../../../../examples/config-file/zarf.yaml?raw", "v0-76"),
    "/src/assets/versioned-assets/v0-76/examples/config-file/zarf.yaml?raw",
  );
  assert.equal(
    rewriteVersionedAssetImport("../../../../../packages/distros/eks/zarf.yaml?raw", "v0-76"),
    "/src/assets/versioned-assets/v0-76/packages/distros/eks/zarf.yaml?raw",
  );
});

test("leaves non-versioned asset repo imports untouched", () => {
  assert.equal(rewriteVersionedAssetImport("../../assets/x.svg", "v0-76"), null);
  assert.equal(rewriteVersionedAssetImport("@components/ExampleYAML.astro", "v0-76"), null);
});

test("leaves relative paths that stay within the subtree untouched", () => {
  const fileDir = path.join(versionRoot, "getting-started");
  assert.equal(fixEscapingRelative("./img.png", fileDir, versionRoot), "./img.png");
  assert.equal(fixEscapingRelative("../tutorials/x", fileDir, versionRoot), "../tutorials/x");
});

test("ignores non-relative specifiers (aliases, bare modules)", () => {
  const fileDir = path.join(versionRoot, "contribute");
  assert.equal(fixEscapingRelative("@components/StripH1.astro", fileDir, versionRoot), "@components/StripH1.astro");
});
