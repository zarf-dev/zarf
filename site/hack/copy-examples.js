import { promises as fs } from "fs";
import path from "path";
import yaml from "yaml";

const __dirname = import.meta.dirname;

const examplesDir = path.join(__dirname, "../../examples");
const dstDir = path.join(__dirname, "../src/content/docs/ref/Examples");

async function preflight() {
  await fs.rm(dstDir, { recursive: true, force: true });
  await fs.mkdir(dstDir, { recursive: true });
}

async function copyExamples() {
  const dirs = await fs.readdir(examplesDir);
  const examples = [];
  for (const dir of dirs) {
    const content = await fs.readFile(path.join(examplesDir, dir, "zarf.yaml"), "utf-8");
    const parsed = yaml.parseDocument(content);
    const mdx = parsed.get("x-mdx");
    if (!parsed.has("x-mdx")) {
      // throw new Error(`No x-mdx field in ${dir}/zarf.yaml`);
      continue;
    }
    examples.push(dir);
    const repo = "https://github.com/defenseunicorns/zarf";
    const link = new URL(`${repo}/edit/main/examples/${dir}/zarf.yaml`).toString();
    const fm = `---
title: "${dir}"
editURL: "${link}"
description: "${parsed.get("description") || ""}"
tableOfContents: false
---

:::note

To view the full example, as well as its dependencies, please visit [examples/${dir}](${repo}/tree/main/examples/${dir}).

:::

`;

    parsed.delete("x-mdx");

    const pkg = parsed.toString();

    const final = `${fm}
${mdx}

## zarf.yaml

\`\`\`yaml
${pkg}
\`\`\`
`;

    await fs.writeFile(path.join(dstDir, `${dir}.mdx`), final);

    // await fs.copyFile(path.join(examplesDir, dir, "zarf.yaml"), path.join(dstDir, `${dir}.yaml`));
    // await fs.copyFile(path.join(examplesDir, dir, "README.md"), path.join(dstDir, `${dir}.mdx`));
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

  await fs.writeFile(path.join(dstDir, `index.mdx`), index);
}

async function main() {
  await preflight();
  await copyExamples();
}

await main().catch((err) => {
  console.error(err);
  process.exit(1);
});
