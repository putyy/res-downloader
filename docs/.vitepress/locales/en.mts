import type { DefaultTheme, LocaleConfig } from 'vitepress'

export const en = {
  label: 'English',
  lang: 'en-US',
  description: 'Official res-downloader documentation: install the cross-platform resource discovery and download tool, capture resources, manage downloads and plugins, and use CLI / MCP automation.',
  themeConfig: {
    nav: [
      { text: 'Guide', link: '/en/guide/getting-started', activeMatch: '^/en/guide/(getting-started|installation|examples|settings|more|troubleshooting)' },
      { text: 'Plugins', link: '/en/guide/plugin-management', activeMatch: '^/en/(guide/plugin-management|development/(plugin|extension-store))' },
      { text: 'CLI / MCP', link: '/en/guide/automation' },
      {
        text: 'Project',
        items: [
          { text: 'Contributing', link: '/en/development/contributing' },
          { text: 'Architecture', link: '/en/development/architecture' },
          { text: 'Release Notes', link: 'https://github.com/putyy/res-downloader/releases' },
          { text: 'Report an Issue', link: 'https://github.com/putyy/res-downloader/issues' },
        ],
      },
    ],
    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Quick Start', link: '/en/guide/getting-started' },
          { text: 'Installation', link: '/en/guide/installation' },
          { text: 'Feature Walkthrough', link: '/en/guide/examples' },
        ],
      },
      {
        text: 'Resources and Downloads',
        items: [
          { text: 'Plugin Management', link: '/en/guide/plugin-management' },
          { text: 'Settings', link: '/en/guide/settings' },
          { text: 'CLI and MCP', link: '/en/guide/automation' },
          { text: 'Tips', link: '/en/guide/more' },
          { text: 'Troubleshooting', link: '/en/guide/troubleshooting' },
        ],
      },
      {
        text: 'Plugin Development',
        items: [
          { text: 'Developer Guide', link: '/en/development/plugins' },
          { text: 'Plugin SDK v1', link: '/en/development/plugin-sdk' },
          { text: 'Publishing to the Extension Store', link: '/en/development/extension-store' },
        ],
      },
      {
        text: 'Contributing',
        items: [
          { text: 'Contribution Guide', link: '/en/development/contributing' },
          { text: 'Architecture', link: '/en/development/architecture' },
        ],
      },
    ],
    langMenuLabel: 'Change language',
    outline: { level: [2, 3], label: 'On this page' },
    docFooter: { prev: 'Previous page', next: 'Next page' },
    sidebarMenuLabel: 'Menu',
    returnToTopLabel: 'Back to top',
    darkModeSwitchLabel: 'Appearance',
    lightModeSwitchTitle: 'Switch to light theme',
    darkModeSwitchTitle: 'Switch to dark theme',
    skipToContentLabel: 'Skip to content',
    notFound: {
      title: 'Page not found',
      quote: 'There is no document at this address. Return home or use search to find what you need.',
      linkLabel: 'Go to home',
      linkText: 'Go to home',
    },
    editLink: {
      pattern: 'https://github.com/putyy/res-downloader/edit/master/docs/:path',
      text: 'Edit this page on GitHub',
    },
    footer: {
      message: 'Make sure you have the rights to the resources you process and comply with local laws, platform terms, and copyright requirements.',
      copyright: 'res-downloader · Windows / macOS / Linux',
    },
  },
} satisfies LocaleConfig<DefaultTheme.Config>[string]

export const enSearch = {
  translations: {
    button: { buttonText: 'Search docs', buttonAriaLabel: 'Search documentation' },
    modal: {
      displayDetails: 'Display details',
      resetButtonTitle: 'Clear search',
      backButtonTitle: 'Close search',
      noResultsText: 'No results found',
      footer: { selectText: 'Select', navigateText: 'Navigate', closeText: 'Close' },
    },
  },
} satisfies Omit<DefaultTheme.LocalSearchOptions, 'locales'>
