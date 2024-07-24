import { themes as prismThemes } from "prism-react-renderer";
import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";
import { includeMarkdown } from "@hashicorp/remark-plugins";
import path from "path";

const config: Config = {
  title: "OpenBao",
  tagline: "Manage, store, and distribute sensitive data",
  favicon: "img/favicon.svg",

  // Set the production url of your site here
  url: "https://openbao.org",
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: "/",
  trailingSlash: true,

  // GitHub pages deployment config.
  // If you aren't using GitHub pages, you don't need these.
  organizationName: "openbao", // Usually your GitHub org/user name.
  projectName: "openbao", // Usually your repo name.

  onBrokenLinks: "warn",
  onBrokenMarkdownLinks: "warn",
  // ignore broken anchors as most of them are false positives
  onBrokenAnchors: "ignore",

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },
  staticDirectories: ["public"],

  presets: [
    [
      "classic",
      {
        docs: {
          sidebarPath: "./sidebars.ts",
          // Please change this to your repo.
          // Remove this to remove the "edit this page" links.
          editUrl: "https://github.com/openbao/openbao/tree/main/website/",
          beforeDefaultRemarkPlugins: [
            [
              includeMarkdown,
              {
                resolveMdx: true,
                resolveFrom: path.join(process.cwd(), "content", "partials"),
              },
            ],
          ],
          path: "content/docs",
        },
        blog: false,
        theme: {
          customCss: "./src/css/custom.css",
        },
      } satisfies Preset.Options,
    ],
  ],
  plugins: [
    [
      "@docusaurus/plugin-content-docs",
      {
        id: "api-docs",
        path: "content/api-docs",
        routeBasePath: "api-docs",
        sidebarPath: "./sidebarsApi.ts",
        editUrl: "https://github.com/openbao/openbao/tree/main/website/",
        beforeDefaultRemarkPlugins: [
          [
            includeMarkdown,
            {
              resolveMdx: true,
              resolveFrom: path.join(process.cwd(), "content", "partials"),
            },
          ],
        ],
      },
    ],
    require.resolve("docusaurus-lunr-search"),
  ],

  themeConfig: {
    announcementBar: {
      id: "support_us",
      content:
        'The documentation is still work in progress. If you find any mistakes, please open an <a href="https://github.com/openbao/openbao/issues" target="_blank">issue</a>',
      backgroundColor: "#ffba00",
      textColor: "#091E42",
      isCloseable: false,
    },
    navbar: {
      title: "OpenBao",
      logo: {
        alt: "OpenBao Logo",
        src: "img/logo-black.svg",
        srcDark: "img/logo-white.svg",
      },
      items: [
        {
          to: "/docs/",
          label: "Docs",
          position: "left",
        },
        { to: "/api-docs/", label: "API", position: "left" },
        {
          type: "dropdown",
          label: "Community",
          position: "left",
          items: [
            {
              label: "GitHub Discussions",
              href: "https://github.com/openbao/openbao/discussions",
            },
            {
              label: "Chat Server",
              href: "https://chat.lfx.linuxfoundation.org/",
            },
            {
              label: "Wiki",
              href: "https://wiki.lfedge.org/display/OH/OpenBao+%28Hashicorp+Vault+Fork+effort%29+FAQ",
            },
          ],
        },
        {
          href: "https://github.com/openbao/openbao",
          label: "GitHub",
          position: "right",
        },
      ],
    },
    footer: {
      copyright: [
        `Copyright © ${new Date().getFullYear()} OpenBao a Series of LF Projects, LLC <br>`,
        `For web site terms of use, trademark policy and other project policies please see <a href="https://lfprojects.org">lfprojects.org</a>. <br>`,
        ` OpenBao is a <a href="https://wiki.lfedge.org/display/LE/Stage+1%3A+At+Large">Stage One project</a> at`,
        `<a href="https://www.lfedge.org/"><img src="/img/lfedge-logo.svg" alt="LF Edge Logo" width="90px"></a>.`,
      ].join(" "),
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
