import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import yaml from "yaml";
import { stageDocs } from "./build-versions.mjs";

test("stages archived pages with version-scoped explicit slugs", async (t) => {
  const root = await mkdtemp(path.join(tmpdir(), "zarf-archived-docs-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const srcDir = path.join(root, "source");
  const dstDir = path.join(root, "v1-0");
  await mkdir(path.join(srcDir, "nested"), { recursive: true });

  const body = '\nimport Timeline from "@components/Timeline.astro";\n\nRelease notes.\n';
  const pages = [
    { filename: "v1.0.mdx", slug: "v1.0", expected: "v1-0/v1.0" },
    { filename: "nested/custom.md", slug: "/custom/path/", expected: "v1-0/custom/path/" },
    { filename: "index.mdx", slug: undefined, expected: undefined },
  ];

  for (const page of pages) {
    page.content = `---\n${yaml.stringify({ title: "Release", slug: page.slug })}---\n${body}`;
    await writeFile(path.join(srcDir, page.filename), page.content);
  }

  await stageDocs({ srcDir, dstDir, slug: "v1-0" });

  for (const page of pages) {
    const staged = await readFile(path.join(dstDir, page.filename), "utf8");
    const frontmatter = staged.match(/^---\n([\s\S]*?)\n---\n/);
    assert.equal(yaml.parse(frontmatter[1]).slug, page.expected);
    assert.equal(staged.slice(frontmatter[0].length), body);
    assert.equal(await readFile(path.join(srcDir, page.filename), "utf8"), page.content);
    if (!page.slug) assert.equal(staged, page.content);
  }
});
