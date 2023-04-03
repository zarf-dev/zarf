// @ts-check
// Note: type annotations allow type checking and IDEs autocompletion

const darkCodeTheme = require('prism-react-renderer/themes/dracula')
const { SocialsBox } = require('./static-components/SocialsBox/SocialsBox')

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Zarf Documentation',
  tagline: 'Airgap is hard. Zarf makes it easy.',
  url: 'https://zarf.dev',
  baseUrl: '/',
  onBrokenLinks: 'warn',
  onBrokenMarkdownLinks: 'warn',
  favicon: 'img/favicon.svg',
  organizationName: 'Defense Unicorns', // Usually your GitHub org/user name.
  projectName: 'Zarf', // Usually your repo name.
  markdown: {
    mermaid: true,
  },
  themes: [
    [require.resolve('@easyops-cn/docusaurus-search-local'), { hashed: true }],
    [require.resolve('@docusaurus/theme-mermaid'), { hashed: true }],
  ],
  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          path: '..',
          include: [
            'CONTRIBUTING.md',
            'adr/**/*.{md,mdx}',
            'docs/**/*.{md,mdx}',
            'examples/**/*.{md,mdx}',
            'packages/**/*.{md,mdx}',
          ],
          sidebarPath: require.resolve('./src/sidebars.js'),
          // The '/x/' at the end if the editUrl is patching a defect in the plugin URL rendering. Removing it will break the base path for editing the docs.
          editUrl: 'https://github.com/defenseunicorns/zarf/tree/main/x/',
          routeBasePath: '/',
          async sidebarItemsGenerator({
            defaultSidebarItemsGenerator,
            ...args
          }) {
            const sidebarItems = await defaultSidebarItemsGenerator(args)
            if (args.item.dirName === 'examples') {
              // This hack removes the "Overview" page from the sidebar on the examples page
              return sidebarItems.slice(1)
            }
            return sidebarItems
          },
        },
        blog: false,
        theme: {
          customCss: [require.resolve('./src/css/custom.css')],
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      colorMode: {
        defaultMode: 'dark',
        disableSwitch: true,
      },
      navbar: {
        logo: {
          alt: 'Zarf',
          src: 'img/zarf-logo-light.svg',
          srcDark: 'img/zarf-logo-dark.svg',
          href: 'https://zarf.dev/',
          target: '_self',
        },
        items: [
          {
            type: 'search',
            position: 'right',
          },
          {
            type: 'doc',
            docId: 'docs/zarf-overview',
            position: 'left',
            label: 'Docs',
          },
          {
            position: 'left',
            label: 'Product',
            to: 'https://zarf.dev',
            target: '_self',
          },
          {
            type: 'html',
            position: 'right',
            className: 'navbar__item--socials-box',
            value: SocialsBox({
              linkClass: 'menu__link',
            }),
          },
        ],
      },
      footer: {
        style: 'dark',
        logo: {
          alt: 'Zarf',
          src: 'img/zarf-logo-light.svg',
          srcDark: 'img/zarf-logo-dark.svg',
          href: 'https://zarf.dev/',
        },
        copyright: `<p class="p-copy">Copyright © ${new Date().getFullYear()} Zarf Project, All rights reserved.</p>`,
        links: [
          {
            html: SocialsBox(),
          },
        ],
      },
      prism: {
        theme: darkCodeTheme,
        darkTheme: darkCodeTheme,
      },
    }),
}

module.exports = config
