import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import { rehypeHeadingIds } from "@astrojs/markdown-remark";
import rehypeAutolinkHeadings from "rehype-autolink-headings";
import remarkGemoji from "remark-gemoji";

// https://astro.build/config
export default defineConfig({
  redirects: {
    '/docs/zarf-overview': '/'
  },
  markdown: {
    remarkPlugins: [remarkGemoji],
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
  },
  integrations: [
    starlight({
      title: "Zarf",
      social: {
        github: "https://github.com/defenseunicorns/zarf",
        slack: "https://kubernetes.slack.com/archives/C03B6BJAUJ3",
      },
      favicon: "/favicon.svg",
      editLink: {
        baseUrl: "https://github.com/defenseunicorns/zarf/edit/main/",
      },
      logo: {
        src: "./src/assets/zarf-logo-header.svg",
        replacesTitle: true,
      },
      customCss: [
        "./src/styles/custom.css",
        "@fontsource/space-grotesk/400.css",
        "@fontsource/source-code-pro/400.css",
      ],
      lastUpdated: true,
      sidebar: [
        {
          label: "Overview",
          link: "/",
        },
        {
          label: "Start Here",
          autogenerate: {
            directory: "getting-started",
          },
        },
        {
          label: "CLI Commands",
          autogenerate: { directory: "commands" },
          collapsed: true,
        },
        {
          label: "Reference",
          autogenerate: { directory: "ref", collapsed: true },
          collapsed: true,
        },
        {
          label: "Tutorials",
          autogenerate: { directory: "tutorials" },
          collapsed: true,
        },
        {
          label: "FAQ",
          link: "/faq",
        },
        {
          label: "Roadmap",
          link: "/roadmap",
        },
        {
          label: "Support",
          link: "/support",
        },
        {
          label: "Contribute",
          autogenerate: { directory: "contribute" },
          collapsed: true,
        },
      ],
    }),
  ],
});
