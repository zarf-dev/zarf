import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import { rehypeHeadingIds } from "@astrojs/markdown-remark";
import rehypeAutolinkHeadings from "rehype-autolink-headings";
import remarkGemoji from "remark-gemoji";

// https://astro.build/config
export default defineConfig({
  redirects: {
    "/docs/zarf-overview": "/",
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
          label: "Best Practices",
          autogenerate: { directory: "best-practices" },
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
