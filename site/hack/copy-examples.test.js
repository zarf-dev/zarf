import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { generateExamples } from "./copy-examples.js";

test("generates documentation for legacy, v1beta1, and template examples", async (t) => {
  const root = await mkdtemp(path.join(tmpdir(), "zarf-example-docs-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const examplesDir = path.join(root, "examples");
  const dstDir = path.join(root, "docs");

  for (const [name, filename, attachmentKey] of [
    ["legacy", "zarf.yaml", "documentation"],
    ["beta", "zarf.yaml", "attachments"],
    ["template", "zarf.tpl.yaml", "attachments"],
  ]) {
    const dir = path.join(examplesDir, name);
    await mkdir(dir, { recursive: true });
    const apiVersion = name === "legacy" ? "v1alpha1" : "v1beta1";
    await writeFile(path.join(dir, filename), `apiVersion: zarf.dev/${apiVersion}
kind: ZarfPackageConfig
metadata:
  name: ${name}
${attachmentKey}:
  readme: readme.md
`);
    await writeFile(path.join(dir, "readme.md"), `Instructions for ${name}.`);
  }

  await generateExamples({ examplesDir, dstDir });

  const index = await readFile(path.join(dstDir, "index.mdx"), "utf8");
  for (const name of ["legacy", "beta", "template"]) {
    const page = await readFile(path.join(dstDir, `${name}.mdx`), "utf8");
    assert.ok(page.includes(`Instructions for ${name}.`));
    assert.ok(page.includes(name === "template" ? "## zarf.tpl.yaml" : "## zarf.yaml"));
    assert.ok(index.includes(`/ref/examples/${name}/`));
  }
});
