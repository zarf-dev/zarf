import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

// https://astro.build/config
export default defineConfig({
  integrations: [
    starlight({
      title: "Zarf",
      social: {
        github: "https://github.com/defenseunicorns/zarf",
        slack: "https://kubernetes.slack.com/archives/C03B6BJAUJ3",
      },
      editLink: {
        baseUrl: "https://github.com/defenseunicorns/zarf/edit/main/",
      },
      logo: {
        src: "./src/assets/zarf-logo-header.svg",
		replacesTitle: true,
      },
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
