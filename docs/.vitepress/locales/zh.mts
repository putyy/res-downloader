import type { DefaultTheme, LocaleConfig } from 'vitepress'

export const zh = {
  label: '简体中文',
  lang: 'zh-CN',
  description: 'res-downloader 官方文档：跨平台网络资源发现与下载工具，提供安装、资源抓取、下载管理、插件和 CLI / MCP 使用指南。',
  themeConfig: {
    nav: [
      { text: '使用指南', link: '/zh/guide/getting-started', activeMatch: '^/zh/guide/(getting-started|installation|examples|settings|more|troubleshooting)' },
      { text: '插件', link: '/zh/guide/plugin-management', activeMatch: '^/zh/(guide/plugin-management|development/(plugin|extension-store))' },
      { text: 'CLI / MCP', link: '/zh/guide/automation' },
      {
        text: '项目',
        items: [
          { text: '参与贡献', link: '/zh/development/contributing' },
          { text: '架构说明', link: '/zh/development/architecture' },
          { text: '更新日志', link: 'https://github.com/putyy/res-downloader/releases' },
          { text: '问题反馈', link: 'https://github.com/putyy/res-downloader/issues' },
        ],
      },
    ],
    sidebar: [
      {
        text: '开始使用',
        items: [
          { text: '快速开始', link: '/zh/guide/getting-started' },
          { text: '安装指南', link: '/zh/guide/installation' },
          { text: '功能演示', link: '/zh/guide/examples' },
        ],
      },
      {
        text: '资源与下载',
        items: [
          { text: '插件管理', link: '/zh/guide/plugin-management' },
          { text: '资源与下载设置', link: '/zh/guide/settings' },
          { text: 'CLI 与 MCP', link: '/zh/guide/automation' },
          { text: '使用技巧', link: '/zh/guide/more' },
          { text: '常见问题', link: '/zh/guide/troubleshooting' },
        ],
      },
      {
        text: '插件开发',
        items: [
          { text: '开发指南', link: '/zh/development/plugins' },
          { text: '插件 SDK v1', link: '/zh/development/plugin-sdk' },
          { text: '发布到扩展商店', link: '/zh/development/extension-store' },
        ],
      },
      {
        text: '参与项目',
        items: [
          { text: '参与贡献', link: '/zh/development/contributing' },
          { text: '架构说明', link: '/zh/development/architecture' },
        ],
      },
    ],
    langMenuLabel: '切换语言',
    outline: { level: [2, 3], label: '本页目录' },
    docFooter: { prev: '上一篇', next: '下一篇' },
    sidebarMenuLabel: '文档导航',
    returnToTopLabel: '返回顶部',
    darkModeSwitchLabel: '外观',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
    skipToContentLabel: '跳转到正文',
    notFound: {
      title: '页面不存在',
      quote: '这个地址没有对应的文档，可以返回首页或使用搜索查找内容。',
      linkLabel: '返回首页',
      linkText: '返回首页',
    },
    editLink: {
      pattern: 'https://github.com/putyy/res-downloader/edit/master/docs/:path',
      text: '在 GitHub 上编辑此页',
    },
    footer: {
      message: '请确保对所处理的资源拥有合法权利，并遵守所在地法律、平台协议和版权规定。',
      copyright: 'res-downloader · Windows / macOS / Linux',
    },
  },
} satisfies LocaleConfig<DefaultTheme.Config>[string]

export const zhSearch = {
  translations: {
    button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
    modal: {
      displayDetails: '显示详细内容',
      resetButtonTitle: '清除搜索',
      backButtonTitle: '关闭搜索',
      noResultsText: '没有找到相关结果',
      footer: { selectText: '选择', navigateText: '切换', closeText: '关闭' },
    },
  },
} satisfies Omit<DefaultTheme.LocalSearchOptions, 'locales'>
