import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

// https://astro.build/config
export default defineConfig({
  vite: {
    resolve: {
      alias: {
        "@examples": "../examples",
        "@packages": "../packages",
      },
    },
  },
  integrations: [
    starlight({
      title: "Zarf",
      social: {
        github: "https://github.com/defenseunicorns/zarf",
        slack: "https://kubernetes.slack.com/archives/C03B6BJAUJ3",
      },
      favicon: "./src/assets/favicon.svg",
      editLink: {
        baseUrl: "https://github.com/defenseunicorns/zarf/edit/main/",
      },
      logo: {
        src: "./src/assets/zarf-logo-header.svg",
        replacesTitle: true,
      },
      lastUpdated: true,
      sidebar: [
        {
          label: "Start Here",
          autogenerate: { directory: "getting-started" },
          collapsed: true,
        },
        {
          label: "CLI",
          autogenerate: { directory: "cli" },
          collapsed: true,
        },
        {
          label: "Create a Package",
          autogenerate: { directory: "create-a-package" },
          collapsed: true,
        },
        {
          label: "Deploy a Package",
          autogenerate: { directory: "deploy-a-package" },
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
          label: "Community",
          link: "/community",
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
