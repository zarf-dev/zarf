import { test } from "node:test";
import assert from "node:assert/strict";
import path from "node:path";
import { rewriteUrl, fixEscapingRelative } from "./remark-link-rewrite.ts";

const sections = new Set(["commands", "ref", "tutorials", "getting-started", "faq"]);
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

// fixEscapingRelative: docs root /docs, version subtree /docs/v0-76
const srcDir = "/docs";
const versionRoot = path.join(srcDir, "v0-76");

test("adds one ../ to asset paths escaping the version subtree", () => {
  // image in /docs/v0-76/index.mdx pointing at shared src/assets
  const fileDir = path.join(versionRoot);
  assert.equal(fixEscapingRelative("../../assets/x.svg", fileDir, versionRoot), "../../../assets/x.svg");
});

test("adds one ../ to escaping ESM/raw imports, preserving query", () => {
  const fileDir = path.join(versionRoot, "ref");
  assert.equal(
    fixEscapingRelative("../../../../../examples/config-file/zarf.yaml?raw", fileDir, versionRoot),
    "../../../../../../examples/config-file/zarf.yaml?raw",
  );
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
