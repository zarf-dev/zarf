import { promises as fs } from "fs";
import path from "path";
import yaml from "yaml";
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const repo = "https://github.com/zarf-dev/zarf";

// Generate `ref/Examples` docs from a checkout's `examples/` directory. The
// orchestrator (build-versions.mjs) calls this per archived version with that
// tag's examples and ref; the default args generate the current checkout's.
export async function generateExamples({
  examplesDir = path.join(__dirname, "../../examples"),
  dstDir = path.join(__dirname, "../src/content/docs/ref/Examples"),
  ref = "main",
} = {}) {
  await fs.rm(dstDir, { recursive: true, force: true });
  await fs.mkdir(dstDir, { recursive: true });

  const dirs = await fs.readdir(examplesDir);
  const examples = [];
  for (const dir of dirs) {
    let content;
    try {
      content = await fs.readFile(path.join(examplesDir, dir, "zarf.yaml"), "utf-8");
    } catch {
      continue;
    }
    const parsed = yaml.parse(content);
    const readmeFile = parsed.documentation?.readme;
    if (!readmeFile) {
      continue;
    }
    const readmePath = path.join(examplesDir, dir, readmeFile);
    try {
      await fs.access(readmePath);
    } catch {
      continue;
    }
    const readme = (await fs.readFile(readmePath, "utf-8")).trim();
    examples.push(dir);
    const link = new URL(`${repo}/edit/${ref}/examples/${dir}/${readmeFile}`).toString();
    const fm = `---
title: "${dir}"
editURL: "${link}"
description: "${parsed.description || ""}"
tableOfContents: false
---

:::note

To view the full example, as well as its dependencies, please visit [examples/${dir}](${repo}/tree/${ref}/examples/${dir}).

:::
`;

    const pkg = content.trim();

    const final = `${fm}
${readme}

## zarf.yaml

\`\`\`yaml
${pkg}
\`\`\`
`.trim();

    await fs.writeFile(path.join(dstDir, `${dir}.mdx`), final + "\n");
  }

  const index = `---
title: "Overview"
description: "Examples of \`zarf.yaml\` configurations"
tableOfContents: false
---

import { LinkCard, CardGrid } from '@astrojs/starlight/components';

<CardGrid>
  ${examples.map((e) => `<LinkCard title="${e}" href="/ref/examples/${e}/" />`).join("\n")}
</CardGrid>
`;

  await fs.writeFile(path.join(dstDir, `index.mdx`), index + "\n");
}

// CLI entry: generate the current checkout's examples (used by prebuild/predev).
if (import.meta.url === `file://${process.argv[1]}`) {
  await generateExamples().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
