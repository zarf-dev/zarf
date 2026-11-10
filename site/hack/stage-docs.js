import { promises as fs } from "node:fs";
import path from "node:path";
import yaml from "yaml";

// Explicit slugs override the directory-based routes, so archived pages must
// have their slugs scoped to the version just like pages without an override.
export async function stageDocs({ srcDir, dstDir, slug }) {
  await fs.rm(dstDir, { recursive: true, force: true });
  await fs.cp(srcDir, dstDir, { recursive: true });

  for (const entry of await fs.readdir(dstDir, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile() || !/\.mdx?$/.test(entry.name)) continue;
    const filename = path.join(entry.parentPath, entry.name);
    const content = await fs.readFile(filename, "utf8");
    const frontmatter = content.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/);
    if (!frontmatter) continue;

    const doc = yaml.parseDocument(frontmatter[1]);
    if (doc.errors.length > 0) throw new Error(`Invalid frontmatter in ${filename}`, { cause: doc.errors[0] });
    const pageSlug = doc.get("slug");
    if (!pageSlug) continue;
    doc.set("slug", path.posix.join(slug, String(pageSlug)));
    await fs.writeFile(filename, `---\n${doc.toString()}---\n${content.slice(frontmatter[0].length)}`);
  }
}
