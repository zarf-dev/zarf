import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import starlightSidebarTopics from "starlight-sidebar-topics";
import { rehypeHeadingIds, unified } from "@astrojs/markdown-remark";
import rehypeAutolinkHeadings from "rehype-autolink-headings";
import remarkGemoji from "remark-gemoji";
import { remarkLinkRewrite } from "./src/plugins/remark-link-rewrite.ts";
import { readFileSync, readdirSync, existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const docsDir = fileURLToPath(new URL("./src/content/docs/", import.meta.url));
const VERSION_SLUG = /^v\d+-\d+$/;

// Archived versions staged by build-versions.mjs; absent in plain/dev builds.
let versions: { ref: string; slug: string }[] = [];
try {
  const raw = readFileSync(new URL("./versions.json", import.meta.url), "utf8");
  versions = JSON.parse(raw).versions ?? [];
} catch {}

// Top-level docs sections (dirs + single pages, excluding staged version subtrees).
// Drives both link-rewrite eligibility and the per-version sidebars below.
const sections = readdirSync(docsDir, { withFileTypes: true })
  .filter((e) => !(e.isDirectory() && VERSION_SLUG.test(e.name)))
  .map((e) => e.name.replace(/\.mdx?$/, ""))
  .filter((name) => name !== "index");

// Build a Starlight sidebar for one docs version. `slug` is "" for the current
// checkout (Latest) or a version slug (e.g. "0-76") for an archived subtree.
// Sections missing from an older version are skipped so autogenerate never
// points at a non-existent directory.
function buildSidebar(slug: string): any[] {
  const base = slug ? `/${slug}` : "";
  const rel = (p: string) => (slug ? `${slug}/${p}` : p);
  const hasDir = (d: string) => existsSync(path.join(docsDir, rel(d)));
  const hasPage = (p: string) =>
    existsSync(path.join(docsDir, rel(`${p}.mdx`))) || existsSync(path.join(docsDir, rel(`${p}.md`)));

  const items: any[] = [{ label: "Overview", link: `${base}/` }];

  const dirGroup = (label: string, d: string, opts: { collapsed?: boolean; innerCollapsed?: boolean } = {}) => {
    if (!hasDir(d)) return;
    const autogenerate: { directory: string; collapsed?: boolean } = { directory: rel(d) };
    if (opts.innerCollapsed) autogenerate.collapsed = true;
    items.push({ label, items: [{ autogenerate }], ...(opts.collapsed ? { collapsed: true } : {}) });
  };
  const pageLink = (label: string, p: string) => {
    if (hasPage(p)) items.push({ label, link: `${base}/${p}` });
  };

  dirGroup("Start Here", "getting-started");
  dirGroup("CLI Commands", "commands", { collapsed: true });
  dirGroup("Best Practices", "best-practices", { collapsed: true });
  dirGroup("Reference", "ref", { collapsed: true, innerCollapsed: true });
  dirGroup("Tutorials", "tutorials", { collapsed: true });
  dirGroup("Schema", "schema", { collapsed: true });
  pageLink("FAQ", "faq");
  pageLink("Roadmap", "roadmap");
  pageLink("Support", "support");
  dirGroup("Contribute", "contribute", { collapsed: true });
  return items;
}

// One topic per version. The topic dropdown (see src/components/Sidebar.astro)
// doubles as the version switcher, and each topic scopes the sidebar to its
// version's subtree.
const topics = [
  { id: "latest", label: "Latest", link: "/", items: buildSidebar("") },
  ...versions.map((v) => ({ id: v.slug, label: v.ref, link: `/${v.slug}/`, items: buildSidebar(v.slug) })),
];

// Associate generated pages that aren't in any sidebar with a topic.
const topicsOption: Record<string, string[]> = { latest: ["/404"] };

// https://astro.build/config
export default defineConfig({
  redirects: {
    "/docs/zarf-overview": "/",
  },
  markdown: {
    processor: unified({
      gfm: true,
      remarkPlugins: [
        remarkGemoji,
        [remarkLinkRewrite, { srcDir: docsDir, sections }],
      ],
      rehypePlugins: [
        rehypeHeadingIds,
        [
          rehypeAutolinkHeadings,
          {
            behavior: "wrap",
            properties: { ariaHidden: true, tabIndex: -1, class: "heading-link" },
          },
        ],
      ],
    }),
  },
  integrations: [
    starlight({
      title: "Zarf",
      // We render our own heading anchors (rehype-autolink-headings); disable
      // Starlight's to avoid duplicates. TODO: switch to native Starlight links.
      markdown: { headingLinks: false },
      head: [
        {
          tag: "script",
          content: `(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':
          new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0],
          j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src=
          'https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f);
          })(window,document,'script','dataLayer','G-N1XZ8ZXCWL');`,
        },
      ],
      components: {
        SkipLink: "./src/components/SkipLink.astro",
        ThemeSelect: "./src/components/ThemeSelect.astro",
        Sidebar: "./src/components/Sidebar.astro",
      },
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/zarf-dev/zarf' },
        { icon: 'slack', label: 'Slack', href: 'https://kubernetes.slack.com/archives/C03B6BJAUJ3' },
      ],
      favicon: "/favicon.svg",
      editLink: {
        baseUrl: "https://github.com/zarf-dev/zarf/edit/main/site",
      },
      logo: {
        src: "./src/assets/zarf-logo-header.svg",
        replacesTitle: true,
      },
      customCss: [
        "./src/styles/custom.css",
        "@fontsource/source-code-pro/400.css",
      ],
      lastUpdated: true,
      plugins: [starlightSidebarTopics(topics, { topics: topicsOption })],
    }),
  ],
});
